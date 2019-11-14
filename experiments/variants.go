package experiments

type VariantSet interface {
	ChooseVariant(bucket int) string
}

type SingleVariantSet struct {
	variants []Variant
	buckets  int
}

func NewSingleVariantSet(variants []Variant, buckets int) *SingleVariantSet {
	return &SingleVariantSet{
		variants: variants,
		buckets:  buckets,
	}
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

func FromExperimentType(experimentType string, variants []Variant, buckets int) VariantSet {
	switch experimentType {
	case "single_variant":
		return NewSingleVariantSet(variants, buckets)
	}
	return nil
}
