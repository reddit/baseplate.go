package experiments

import "fmt"

type VariantSet interface {
	ChooseVariant(bucket int) string
	validate(variants []Variant) error
}

type SingleVariantSet struct {
	variants []Variant
	buckets  int
}

func NewSingleVariantSet(variants []Variant, buckets int) (*SingleVariantSet, error) {
	variantSet := &SingleVariantSet{
		variants: variants,
		buckets:  buckets,
	}
	err := variantSet.validate(variants)
	if err != nil {
		return nil, err
	}
	return variantSet, nil
}

func (v *SingleVariantSet) validate(variants []Variant) error {
	if variants == nil {
		return VariantValidationError("no variants provided")
	}
	if len(variants) != 2 {
		return VariantValidationError("Single Variant experiments expects only one variant and one control")
	}
	// TODO figure out if parsing of null float should be allowed
	totalSize := variants[0].Size + variants[1].Size
	if totalSize < 0.0 || totalSize > 1.0 {
		return VariantValidationError("sum of all variants must be between 0 and 1")
	}
	return nil
}

// ChooseVariant deterministically chooses a variant. Every call with the same
// bucket on on einstance will result in the same answer.
func (v *SingleVariantSet) ChooseVariant(bucket int) string {
	if bucket < int(v.variants[0].Size*float64(v.buckets)) {
		return v.variants[0].Name
	}
	if bucket >= v.buckets-int(v.variants[1].Size*float64(v.buckets)) {
		return v.variants[1].Name
	}
	return ""
}

func FromExperimentType(experimentType string, variants []Variant, buckets int) (VariantSet, error) {
	switch experimentType {
	case "single_variant":
		return NewSingleVariantSet(variants, buckets)
	case "multi_variant":
		return NewMultiVariantSet(variants, buckets)
	case "feature_rollout":
		return NewRolloutVariantSet(variants, buckets)
	case "range_variant":
		return NewRangeVariantSet(variants, buckets)
	}
	return nil, fmt.Errorf("experiment type %s unknown", experimentType)
}

type MultiVariantSet struct {
	variants []Variant
	buckets  int
}

func NewMultiVariantSet(variants []Variant, buckets int) (*MultiVariantSet, error) {
	variantSet := &MultiVariantSet{
		variants: variants,
		buckets:  buckets,
	}
	err := variantSet.validate(variants)
	if err != nil {
		return nil, err
	}
	return variantSet, nil
}

func (v *MultiVariantSet) validate(variants []Variant) error {
	if variants == nil {
		return VariantValidationError("no variants provided")
	}
	if len(variants) < 3 {
		return VariantValidationError("Multi Variant experiments expects three or more variants")
	}
	totalSize := 0.0
	for _, variant := range variants {
		totalSize += variant.Size * float64(v.buckets)
	}
	if totalSize > float64(v.buckets) {
		return VariantValidationError("sum of all variants is greater than 100%")
	}
	return nil
}

func (v *MultiVariantSet) ChooseVariant(bucket int) string {
	currentOffset := 0
	for _, variant := range v.variants {
		currentOffset += int(variant.Size * float64(v.buckets))
		if bucket < currentOffset {
			return variant.Name
		}
	}
	return ""
}

type RolloutVariantSet struct {
	variant Variant
	buckets int
}

func NewRolloutVariantSet(variants []Variant, buckets int) (*RolloutVariantSet, error) {
	variantSet := &RolloutVariantSet{
		buckets: buckets,
	}
	err := variantSet.validate(variants)
	if err != nil {
		return nil, err
	}
	variantSet.variant = variants[0]
	return variantSet, nil
}

func (v *RolloutVariantSet) validate(variants []Variant) error {
	if variants == nil {
		return VariantValidationError("no variants provided")
	}
	if len(variants) != 1 {
		return VariantValidationError("Rollout Variant experiments only supports one variant")
	}
	size := variants[0].Size
	if size < 0.0 || size > 1.0 {
		return VariantValidationError("variant size must be between 0 and 1")
	}
	return nil
}

func (v *RolloutVariantSet) ChooseVariant(bucket int) string {
	if bucket < int(v.variant.Size*float64(v.buckets)) {
		return v.variant.Name
	}
	return ""
}

type RangeVariantSet struct {
	variants []Variant
	buckets  int
}

func NewRangeVariantSet(variants []Variant, buckets int) (*RangeVariantSet, error) {
	variantSet := &RangeVariantSet{
		variants: variants,
		buckets:  buckets,
	}
	err := variantSet.validate(variants)
	if err != nil {
		return nil, err
	}
	return variantSet, nil
}

func (v *RangeVariantSet) validate(variants []Variant) error {
	if len(variants) == 0 {
		return VariantValidationError("no variants provided")
	}
	totalSize := 0
	for _, variant := range v.variants {
		rangeSize := variant.RangeEnd - variant.RangeStart
		totalSize += int(rangeSize * float64(v.buckets))
	}
	if totalSize > v.buckets {
		return VariantValidationError("sum of all variants is greater than 100%")
	}
	return nil
}

func (v *RangeVariantSet) ChooseVariant(bucket int) string {
	for _, variant := range v.variants {
		lowerBucket := int(variant.RangeStart * float64(v.buckets))
		upperBucket := int(variant.RangeEnd * float64(v.buckets))
		if lowerBucket <= bucket && bucket < upperBucket {
			return variant.Name
		}
	}
	return ""
}

type VariantValidationError string

func (cause VariantValidationError) Error() string {
	return string(cause)
}
