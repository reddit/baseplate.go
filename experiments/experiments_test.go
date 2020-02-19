package experiments

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"
)

var simpleConfig = &ExperimentConfig{
	ID:             1,
	Name:           "test_experiment",
	Owner:          "test",
	Type:           "single_variant",
	Version:        "1",
	StartTimestamp: time.Now().Add(-30 * 24 * time.Hour).Unix(),
	StopTimestamp:  time.Now().Add(30 * 24 * time.Hour).Unix(),
	Enabled:        func() *bool { b := true; return &b }(),
	Experiment: ParsedExperiment{
		BucketSeed: "some new seed",
		Variants: []Variant{
			Variant{
				Name: "variant_1",
				Size: 0.1,
			},
			Variant{
				Name: "variant_2",
				Size: 0.1,
			},
		},
		ExperimentVersion: 1,
	},
}

func TestCalculateBucketValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		config         *ExperimentConfig
		id             string
		name           string
		shuffleVersion string
		expectedBucket int
	}{
		{
			config: &ExperimentConfig{
				ID:   1,
				Name: "test_experiment",
				Experiment: ParsedExperiment{
					ShuffleVersion: "",
					BucketSeed:     "some new seed",
					Variants: []Variant{
						Variant{
							Name: "variant_1",
							Size: 0.1,
						},
						Variant{
							Name: "variant_2",
							Size: 0.1,
						},
					},
				},
				Type: "single_variant",
			},
			expectedBucket: 924,
		},
	}
	for _, tt := range tests {
		experiment, err := NewSimpleExperiment(tt.config)
		if err != nil {
			t.Fatal(err)
		}
		experiment.numBuckets = 1000
		bucket := experiment.calculateBucket("t2_1")
		if bucket != tt.expectedBucket {
			t.Errorf("expected %d, actual: %d", tt.expectedBucket, bucket)
		}
	}
}

func TestCalculateBucket(t *testing.T) {
	t.Parallel()
	config := &ExperimentConfig{
		ID:             1,
		Name:           "test_experiment",
		Owner:          "test",
		Type:           "single_variant",
		Version:        "1",
		StartTimestamp: time.Now().Add(-30 * 24 * time.Hour).Unix(),
		StopTimestamp:  time.Now().Add(30 * 24 * time.Hour).Unix(),
		Enabled:        func() *bool { b := true; return &b }(),
		Experiment: ParsedExperiment{
			Variants: []Variant{
				Variant{
					Name: "variant_1",
					Size: 0.1,
				},
				Variant{
					Name: "variant_2",
					Size: 0.1,
				},
			},
			ExperimentVersion: 1,
		},
	}
	experiment, err := NewSimpleExperiment(config)
	if err != nil {
		t.Fatal(err)
	}

	users := experiment.numBuckets * 2000
	var names []string
	for i := 0; i < users; i++ {
		names = append(names, fmt.Sprintf("t2_%d", i))
	}

	counter := make(map[int]int)
	for _, name := range names {
		bucket := experiment.calculateBucket(name)
		counter[bucket]++
		// ensure bucketing is deterministic
		bucketCheck := experiment.calculateBucket(name)
		if bucket != bucketCheck {
			t.Errorf("expected %d, actual: %d", bucket, bucketCheck)
		}
	}
	for i := 0; i < experiment.numBuckets; i++ {
		expected := users / experiment.numBuckets
		actual := counter[i]
		percentEqual := float64(actual) / float64(expected)
		d, ok := almostEqual(percentEqual, 1.0, 0.10)
		if !ok {
			t.Errorf("bucket %d: %f", i, d)
		}
	}
}

func TestCalculateBucketWithSeed(t *testing.T) {
	t.Parallel()

	config := &ExperimentConfig{
		ID:             1,
		Name:           "test_experiment",
		Owner:          "test",
		Type:           "single_variant",
		Version:        "1",
		StartTimestamp: time.Now().Add(-30 * 24 * time.Hour).Unix(),
		StopTimestamp:  time.Now().Add(30 * 24 * time.Hour).Unix(),
		Enabled:        func() *bool { b := true; return &b }(),
		Experiment: ParsedExperiment{
			BucketSeed: "some new seed",
			Variants: []Variant{
				Variant{
					Name: "variant_1",
					Size: 0.1,
				},
				Variant{
					Name: "variant_2",
					Size: 0.1,
				},
			},
			ExperimentVersion: 1,
		},
	}
	experiment, err := NewSimpleExperiment(config)
	if err != nil {
		t.Fatal(err)
	}

	users := experiment.numBuckets * 2000
	var names []string
	for i := 0; i < users; i++ {
		names = append(names, fmt.Sprintf("t2_%d", i))
	}

	counter := make(map[int]int)
	bucketingChanged := false
	for _, name := range names {
		if experiment.bucketSeed != "some new seed" {
			t.Fatalf("expected seed %s, actual: %s", "some new seed", experiment.bucketSeed)
		}
		bucket1 := experiment.calculateBucket(name)
		counter[bucket1]++
		// ensure bucketing is deterministic
		bucketCheck := experiment.calculateBucket(name)
		if bucket1 != bucketCheck {
			t.Errorf("expected %d, actual: %d", bucket1, bucketCheck)
		}

		currentSeed := experiment.bucketSeed
		experiment.bucketSeed = "newstring"
		bucket2 := experiment.calculateBucket(name)
		experiment.bucketSeed = currentSeed

		// Check that the bucketing changed at some point. Can't compare
		// bucket1 to bucket2 inline because sometimes the user will fall into
		// both buckets and the test will fail.
		if bucket1 != bucket2 {
			bucketingChanged = true
		}
	}

	if !bucketingChanged {
		t.Fatal("expected bucketing to change")
	}

	for i := 0; i < experiment.numBuckets; i++ {
		expected := users / experiment.numBuckets
		actual := counter[i]
		percentEqual := float64(actual) / float64(expected)
		d, ok := almostEqual(percentEqual, 1.0, 0.10)
		if !ok {
			t.Errorf("bucket %d: %f", i, d)
		}
	}
}

func TestVariantReturnsNilIfOutOfTimeWindow(t *testing.T) {
	validExperiment, err := NewSimpleExperiment(simpleConfig)
	if err != nil {
		t.Fatal(err)
	}
	expiredExperiment, err := NewSimpleExperiment(simpleConfig)
	if err != nil {
		t.Fatal(err)
	}
	expiredExperiment.endTime = time.Now().Add(-5 * 24 * time.Hour)
	unstartedExperiment, err := NewSimpleExperiment(simpleConfig)
	if err != nil {
		t.Fatal(err)
	}
	unstartedExperiment.startTime = time.Now().Add(5 * 24 * time.Hour)

	validVariant, err := validExperiment.Variant(map[string]interface{}{"user_id": "t2_1"})
	if err != nil {
		t.Error(err)
	}
	if validVariant == "" {
		t.Fatal("expected variant to be not nil")
	}
	expiredVariant, err := expiredExperiment.Variant(map[string]interface{}{"user_id": "t2_1"})
	if err != nil {
		t.Fatal(err)
	}
	if expiredVariant != "" {
		t.Fatal("expected variant to be nil")
	}

	unstartedVariant, err := unstartedExperiment.Variant(map[string]interface{}{"user_id": "t2_1"})
	if err != nil {
		t.Fatal(err)
	}
	if unstartedVariant != "" {
		t.Fatal("expected variant to be nil")
	}
}

func TestNoBucketVal(t *testing.T) {
	experiment, err := NewSimpleExperiment(simpleConfig)
	if err != nil {
		t.Fatal(err)
	}
	result, err := experiment.Variant(map[string]interface{}{"not_user_id": "t2_1"})
	expectedErr := "must specify user_id in call to variant for experiment test_experiment"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("expected error %s but was: %v", expectedErr, err)
	}
	if result != "" {
		t.Errorf("expected result to be empty but was %s", result)
	}

	experiment, err = NewSimpleExperiment(simpleConfig)
	if err != nil {
		t.Fatal(err)
	}
	result, err = experiment.Variant(map[string]interface{}{"not_user_id": ""})
	expectedErr = "must specify user_id in call to variant for experiment test_experiment"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("expected error %s but was: %v", expectedErr, err)
	}
	if result != "" {
		t.Errorf("expected result to be empty but was %s", result)
	}
}

func TestExperimentDisabled(t *testing.T) {
	config := *simpleConfig
	b := false
	config.Enabled = &b
	experiment, err := NewSimpleExperiment(&config)
	if err != nil {
		t.Fatal(err)
	}
	variant, err := experiment.Variant(map[string]interface{}{"user_id": "t2_2"})
	if err != nil {
		t.Error(err)
	}
	if variant != "" {
		t.Errorf("expected variant to be empty but is %s", variant)
	}
}

func TestChangeShuffleVersionChangesBucketing(t *testing.T) {
	shuffleConfig := *simpleConfig
	shuffleConfig.Experiment.BucketSeed = ""
	shuffleConfig.Experiment.ShuffleVersion = "2"

	experiment1, err := NewSimpleExperiment(simpleConfig)
	if err != nil {
		t.Fatal(err)
	}
	experiment2, err := NewSimpleExperiment(&shuffleConfig)
	if err != nil {
		t.Fatal(err)
	}

	users := experiment1.numBuckets * 1000
	var names []string
	for i := 0; i < users; i++ {
		names = append(names, fmt.Sprintf("t2_%d", i))
	}

	bucketingChanged := false
	for _, name := range names {
		bucket1 := experiment1.calculateBucket(name)
		bucket2 := experiment2.calculateBucket(name)
		if bucket1 != bucket2 {
			bucketingChanged = true
			break
		}
	}
	if !bucketingChanged {
		t.Error("expected bucketing to change but did not")
	}
}

func TestOverride(t *testing.T) {
	t.Parallel()
	config := &ExperimentConfig{
		ID:             1,
		Name:           "test_experiment",
		Owner:          "test",
		Type:           "single_variant",
		Version:        "1",
		StartTimestamp: time.Now().Add(-30 * 24 * time.Hour).Unix(),
		StopTimestamp:  time.Now().Add(30 * 24 * time.Hour).Unix(),
		Enabled:        func() *bool { b := true; return &b }(),
		Experiment: ParsedExperiment{
			Variants: []Variant{
				Variant{
					Name: "variant_1",
					Size: 0.1,
				},
				Variant{
					Name: "variant_2",
					Size: 0.1,
				},
			},
			ExperimentVersion: 1,
			Overrides: []map[string]json.RawMessage{
				map[string]json.RawMessage{
					"override_variant_1": []byte(`{"EQ": {"field": "user_id", "value": "t2_1"}}`),
				}},
		},
	}
	experiment, err := NewSimpleExperiment(config)
	if err != nil {
		t.Fatal(err)
	}
	variant, err := experiment.Variant(map[string]interface{}{"user_id": "t2_1"})
	if err != nil {
		t.Fatal(err)
	}
	if variant != "override_variant_1" {
		t.Errorf("expected %s, actual: %s", "override_variant_1", variant)
	}
	variant, err = experiment.Variant(map[string]interface{}{"user_id": "t2_123"})
	if err != nil {
		t.Fatal(err)
	}
	if variant != "variant_1" && variant != "variant_2" {
		t.Errorf("expected %s or %s, actual: %s", "variant_1", "variant_2", variant)
	}
}

func almostEqual(a, b, epsilon float64) (float64, bool) {
	if a == b {
		return 0.0, true
	}
	diff := math.Abs(a - b)
	if diff <= epsilon {
		return diff, true
	}
	round := roundTo(diff, 7)
	if round == 0.0 {
		return diff, true
	}
	return round, false
}

func roundTo(num float64, digits int) float64 {
	shift := math.Pow(10, float64(digits))
	return math.Round(num*shift) / shift
}
