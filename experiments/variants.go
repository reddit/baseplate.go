package experiments

type VariantSet interface {
	ChooseVariant(bucket int) string
	validateVariants() error
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
	err := variantSet.validateVariants()
	if err != nil {
		return nil, err
	}
	return variantSet, nil
}

func (v *SingleVariantSet) validateVariants() error {
	if v.variants == nil {
		return VariantValidationError("no variants provided")
	}
	if len(v.variants) != 2 {
		return VariantValidationError("Single Variant experiments expects only one variant and one control")
	}
	// TODO figure out if parsing of null float should be allowed
	totalSize := v.variants[0].Size + v.variants[1].Size
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
	}
	return nil, nil
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
	err := variantSet.validateVariants()
	if err != nil {
		return nil, err
	}
	return variantSet, nil
}

func (v *MultiVariantSet) validateVariants() error {
	if v.variants == nil {
		return VariantValidationError("no variants provided")
	}
	if len(v.variants) < 3 {
		return VariantValidationError("Multi Variant experiments expects three or more variants")
	}
	totalSize := 0.0
	for _, variant := range v.variants {
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

type VariantValidationError string

func (cause VariantValidationError) Error() string {
	return string(cause)
}
