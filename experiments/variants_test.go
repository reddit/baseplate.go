package experiments

import (
	"errors"
	"testing"
)

func singleVariantConfig() []Variant {
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

func TestSingleVariantSetValidation(t *testing.T) {
	_, err := NewSingleVariantSet(singleVariantConfig(), 1000)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSingleVariantSetValidationFailure(t *testing.T) {
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
				Variant{Name: "variant_2", Size: 0.75},
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
			variantConfig: singleVariantConfig(),
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
				t.Errorf("expected variant_2 to have count %d, actual: %d", tt.variant2Count, variantCounts["variant_2"])
			}
			if variantCounts[""] != tt.emptyCount {
				t.Errorf("expected empty variant to have count %d, actual: %d", tt.emptyCount, variantCounts[""])
			}
		})
	}
}

func TestMultiVariantSetValidation(t *testing.T) {
	_, err := NewMultiVariantSet(multiVariantConfig(), 1000)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMultiVariantSetValidationFails(t *testing.T) {
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
			name: "two variants",
			variants: []Variant{
				Variant{Name: "variant_1", Size: 0.25},
				Variant{Name: "variant_2", Size: 0.25}},
		},
		{
			name: "size too big",
			variants: []Variant{
				Variant{Name: "variant_1", Size: 0.75},
				Variant{Name: "variant_2", Size: 0.75},
				Variant{Name: "variant_3", Size: 0.25},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewMultiVariantSet(tt.variants, 1000)
			var expectedError VariantValidationError
			if !errors.As(err, &expectedError) {
				t.Errorf("expected error %T, actual: %v (%T)", expectedError, err, err)
			}
		})
	}
}

func TestMultiVariantSetDistribution(t *testing.T) {
	tests := []struct {
		name             string
		variantConfig    []Variant
		numBuckets       int
		expectedVariants int
		variant1Count    int
		variant2Count    int
		variant3Count    int
		variant4Count    int
		emptyCount       int
	}{
		{
			name:             "default buckets",
			variantConfig:    multiVariantConfig(),
			numBuckets:       1000,
			expectedVariants: 4,
			variant1Count:    250,
			variant2Count:    250,
			variant3Count:    250,
			emptyCount:       250,
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
				Variant{
					Name: "variant_3",
					Size: 0.0,
				},
			},
			numBuckets:       1000,
			expectedVariants: 4,
			variant1Count:    1,
			variant2Count:    0,
			emptyCount:       999,
		},
		{
			name: "default odd",
			variantConfig: []Variant{
				Variant{
					Name: "variant_1",
					Size: 0.25,
				},
				Variant{
					Name: "variant_2",
					Size: 0.25,
				},
				Variant{
					Name: "variant_3",
					Size: 0.25,
				},
				Variant{
					Name: "variant_4",
					Size: 0.25,
				},
			},
			numBuckets:       1037,
			expectedVariants: 5,
			variant1Count:    259,
			variant2Count:    259,
			variant3Count:    259,
			variant4Count:    259,
			emptyCount:       1,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			variantSet, err := NewMultiVariantSet(tt.variantConfig, tt.numBuckets)
			if err != nil {
				t.Fatal(err)
			}
			variantCounts := map[string]int{
				"variant_1": 0,
				"variant_2": 0,
				"variant_3": 0,
				"":          0,
			}
			for i := 0; i < tt.numBuckets; i++ {
				variant := variantSet.ChooseVariant(i)
				variantCounts[variant] += 1
			}
			if len(variantCounts) != tt.expectedVariants {
				t.Errorf("expected %d variants, actual %d: %v", tt.expectedVariants, len(variantCounts), variantCounts)
			}
			if variantCounts["variant_1"] != tt.variant1Count {
				t.Errorf("expected variant_1 to have count %d, actual: %d", tt.variant1Count, variantCounts["variant_1"])
			}
			if variantCounts["variant_2"] != tt.variant2Count {
				t.Errorf("expected variant_2 to have count %d, actual: %d", tt.variant2Count, variantCounts["variant_2"])
			}
			if variantCounts["variant_3"] != tt.variant3Count {
				t.Errorf("expected variant_3 to have count %d, actual: %d", tt.variant3Count, variantCounts["variant_3"])
			}
			if variantCounts["variant_4"] != tt.variant4Count {
				t.Errorf("expected variant_4 to have count %d, actual: %d", tt.variant4Count, variantCounts["variant_4"])
			}
			if variantCounts[""] != tt.emptyCount {
				t.Errorf("expected empty variant to have count %d, actual: %d", tt.emptyCount, variantCounts[""])
			}
		})
	}
}

func multiVariantConfig() []Variant {
	return []Variant{
		Variant{
			Name: "variant_1",
			Size: 0.25,
		},
		Variant{
			Name: "variant_2",
			Size: 0.25,
		},
		Variant{
			Name: "variant_3",
			Size: 0.25,
		},
	}
}

func TestRolloutVariantSetValidation(t *testing.T) {
	_, err := NewRolloutVariantSet(rolloutVariantConfig(), 1000)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRolloutVariantSetValidationFailure(t *testing.T) {
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
			name: "two variants",
			variants: []Variant{
				Variant{Name: "variant_1", Size: 0.25},
				Variant{Name: "variant_2", Size: 0.25}},
		},
		{
			name: "size too big",
			variants: []Variant{
				Variant{Name: "variant_1", Size: 1.05},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewRolloutVariantSet(tt.variants, 1000)
			var expectedError VariantValidationError
			if !errors.As(err, &expectedError) {
				t.Errorf("expected error %T, actual: %v (%T)", expectedError, err, err)
			}
		})
	}
}

func TestRolloutVariantSetDistribution(t *testing.T) {
	tests := []struct {
		name          string
		variantConfig []Variant
		numBuckets    int
		variantCount  int
		emptyCount    int
	}{
		{
			name:          "default buckets",
			variantConfig: rolloutVariantConfig(),
			numBuckets:    1000,
			variantCount:  250,
			emptyCount:    750,
		},
		{
			name: "single bucket",
			variantConfig: []Variant{
				Variant{
					Name: "variant_1",
					Size: 0.001,
				},
			},
			numBuckets:   1000,
			variantCount: 1,
			emptyCount:   999,
		},
		{
			name: "default odd",
			variantConfig: []Variant{
				Variant{
					Name: "variant_1",
					Size: 1.0,
				},
			},
			numBuckets:   1037,
			variantCount: 1037,
			emptyCount:   0,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			variantSet, err := NewRolloutVariantSet(tt.variantConfig, tt.numBuckets)
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
			if variantCounts["variant_1"] != tt.variantCount {
				t.Errorf("expected variant_1 to have count %d, actual: %d", tt.variantCount, variantCounts["variant_1"])
			}
			if variantCounts[""] != tt.emptyCount {
				t.Errorf("expected empty variant to have count %d, actual: %d", tt.emptyCount, variantCounts[""])
			}
		})
	}
}

func rolloutVariantConfig() []Variant {
	return []Variant{
		Variant{
			Name: "variant_1",
			Size: 0.25,
		},
	}
}
