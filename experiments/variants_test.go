package experiments

import (
	"errors"
	"testing"
)

func TestSingleVariantSet(t *testing.T) {
	t.Run("validation passes", func(t *testing.T) {
		_, err := NewSingleVariantSet(variantConfig(), 1000)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestSingleVariantSetValidationFails(t *testing.T) {
	tests := []struct {
		name     string
		variants []Variant
	}{
		{
			name:     "nil",
			variants: nil,
		},
		{
			name:     "empty",
			variants: []Variant{},
		},
		{
			name:     "one variant",
			variants: []Variant{Variant{Name: "variant_1", Size: 0.25}},
		},
		{
			name: "size too big",
			variants: []Variant{
				Variant{Name: "variant_1", Size: 0.75},
				Variant{Name: "variant_1", Size: 0.75},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewSingleVariantSet(tt.variants, 1000)
			var expectedError VariantValidationError
			if !errors.As(err, &expectedError) {
				t.Errorf("expected error %T, actual: %v (%T)", expectedError, err, err)
			}
		})
	}
}

func TestSingleVariantSetDistribution(t *testing.T) {
	tests := []struct {
		name          string
		variantConfig []Variant
		numBuckets    int
		variant1Count int
		variant2Count int
		emptyCount    int
	}{
		{
			name:          "default buckets",
			variantConfig: variantConfig(),
			numBuckets:    1000,
			variant1Count: 250,
			variant2Count: 250,
			emptyCount:    500,
		},
		{
			name: "single bucket",
			variantConfig: []Variant{
				Variant{
					Name: "variant_1",
					Size: 0.001,
				},
				Variant{
					Name: "variant_2",
					Size: 0.0,
				},
			},
			numBuckets:    1000,
			variant1Count: 1,
			variant2Count: 0,
			emptyCount:    999,
		},
		{
			name: "default odd",
			variantConfig: []Variant{
				Variant{
					Name: "variant_1",
					Size: 0.5,
				},
				Variant{
					Name: "variant_2",
					Size: 0.5,
				},
			},
			numBuckets:    1037,
			variant1Count: 518,
			variant2Count: 518,
			emptyCount:    1,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			variantSet, err := NewSingleVariantSet(tt.variantConfig, tt.numBuckets)
			if err != nil {
				t.Fatal(err)
			}
			variantCounts := map[string]int{
				"variant_1": 0,
				"variant_2": 0,
				"":          0,
			}
			for i := 0; i < tt.numBuckets; i++ {
				variant := variantSet.ChooseVariant(i)
				variantCounts[variant] += 1
			}
			if len(variantCounts) != 3 {
				t.Errorf("expected %d variants, actual %d: %v", 3, len(variantCounts), variantCounts)
			}
			if variantCounts["variant_1"] != tt.variant1Count {
				t.Errorf("expected variant_1 to have count %d, actual: %d", tt.variant1Count, variantCounts["variant_1"])
			}
			if variantCounts["variant_2"] != tt.variant2Count {
				t.Errorf("expected variant_1 to have count %d, actual: %d", tt.variant2Count, variantCounts["variant_2"])
			}
			if variantCounts[""] != tt.emptyCount {
				t.Errorf("expected empty variant to have count %d, actual: %d", tt.emptyCount, variantCounts[""])
			}
		})
	}
}

func variantConfig() []Variant {
	return []Variant{
		Variant{
			Name: "variant_1",
			Size: 0.25,
		},
		Variant{
			Name: "variant_2",
			Size: 0.25,
		},
	}
}
