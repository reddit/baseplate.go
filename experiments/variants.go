package experiments

import "fmt"

// VariantSet is the base interface for variant sets. A variant set contains a
// set of experimental variants, as well as their distributions. It is used by
// experiments to track which bucket a variant is assigned to.
type VariantSet interface {
	ChooseVariant(bucket int) string
	validate(variants []Variant) error
}

// SingleVariantSet Variant Set designed to handle two total treatments.
//
// This variant set allows adjusting the sizes of variants without changing
// treatments, where possible. When not possible (eg: switching from a 60/40
// distribution to a 40/60 distribution), this will minimize changing
// treatments (in the above case, only those buckets between the 40th and 60th
// percentile of the bucketing range will see a change in treatment).
type SingleVariantSet struct {
	variants []Variant
	buckets  int
}

// NewSingleVariantSet returns a new instance of SingleVariantSet based on the
// given variants and number of buckets.
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

// FromExperimentType maps the experimentType to a concrete type implementing
// VariantSet and returns an error for any unknown type.
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

// MultiVariantSet is designed to handle more than two total treatments.
//
// MultiVariantSets are not designed to support changes in variant sizes
// without rebucketing.
type MultiVariantSet struct {
	variants []Variant
	buckets  int
}

// NewMultiVariantSet returns a new instance of MultiVariantSet based on the
// given variants and number of buckets.
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

// ChooseVariant deterministically chooses a variant. Every call with the same
// bucket on one instance will result in the same answer.
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

// RolloutVariantSet is designed for feature rollouts and takes a single
// variant.
//
// Changing the size of the variant will minimize the treatment of bucketed
// users. Those users going from no treatment to the provided treatment (or
// vice versa) are limited to the change in the provided treatment size. For
// instance, going from 45% to 55% will result in only the new 10% of users
// changing treatments. The initial 45% will not change. Conversely, going from
// 55% to 45% will result in only 10% of users losing the treatment.
type RolloutVariantSet struct {
	variant Variant
	buckets int
}

// NewRolloutVariantSet returns a new instance of RolloutVariantSet based on
// the given variants and number of buckets.
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

// ChooseVariant deterministically choose a percentage-based variant. Every
// call with the same bucket and variants will result in the same answer.
func (v *RolloutVariantSet) ChooseVariant(bucket int) string {
	if bucket < int(v.variant.Size*float64(v.buckets)) {
		return v.variant.Name
	}
	return ""
}

// RangeVariantSet is designed to take fixed bucket ranges.
//
// This VariantSet allows manually setting bucketing ranges. It takes in a
// variant name, then the range of buckets in that should be assigned to that
// variant. This enables user-defined bucketing algorithms, as well as
// simplifies the ability to adjust range sizes in special circumstances.
type RangeVariantSet struct {
	variants []Variant
	buckets  int
}

// NewRangeVariantSet returns a new instance of RangeVariantSet based on the
// given variants and number of buckets.
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

// ChooseVariant deterministically choose a variant. Every call with the same
// bucket on one instance will result in the same answer
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

// VariantValidationError is used when the provided variants are not consistent
// with the chosen variant set.
type VariantValidationError string

func (cause VariantValidationError) Error() string {
	return "experiments: " + string(cause)
}
