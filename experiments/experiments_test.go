package experiments

import (
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
		experiment := NewSimpleExperiment(tt.config)
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
	experiment := NewSimpleExperiment(config)

	users := experiment.numBuckets * 2000
	var names []string
	for i := 0; i < users; i++ {
		names = append(names, fmt.Sprintf("t2_%d", i))
	}

	counter := make(map[int]int, 0)
	for _, name := range names {
		bucket := experiment.calculateBucket(name)
		counter[bucket] += 1
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
	experiment := NewSimpleExperiment(config)

	users := experiment.numBuckets * 2000
	var names []string
	for i := 0; i < users; i++ {
		names = append(names, fmt.Sprintf("t2_%d", i))
	}

	counter := make(map[int]int, 0)
	bucketingChanged := false
	for _, name := range names {
		if experiment.seed != "some new seed" {
			t.Fatalf("expected seed %s, actual: %s", "some new seed", experiment.seed)
		}
		bucket1 := experiment.calculateBucket(name)
		counter[bucket1] += 1
		// ensure bucketing is deterministic
		bucketCheck := experiment.calculateBucket(name)
		if bucket1 != bucketCheck {
			t.Errorf("expected %d, actual: %d", bucket1, bucketCheck)
		}

		currentSeed := experiment.seed
		experiment.seed = "newstring"
		bucket2 := experiment.calculateBucket(name)
		experiment.seed = currentSeed

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
	validExperiment := NewSimpleExperiment(simpleConfig)

	expiredExperiment := NewSimpleExperiment(simpleConfig)
	expiredExperiment.endTime = time.Now().Add(-5 * 24 * time.Hour)

	unstartedExperiment := NewSimpleExperiment(simpleConfig)
	unstartedExperiment.startTime = time.Now().Add(5 * 24 * time.Hour)

	validVariant, err := validExperiment.Variant(map[string]string{"user_id": "t2_1"})
	if err != nil {
		t.Error(err)
	}
	if validVariant == "" {
		t.Fatal("expected variant to be not nil")
	}
	expiredVariant, err := expiredExperiment.Variant(map[string]string{"user_id": "t2_1"})
	if err != nil {
		t.Fatal(err)
	}
	if expiredVariant != "" {
		t.Fatal("expected variant to be nil")
	}

	unstartedVariant, err := unstartedExperiment.Variant(map[string]string{"user_id": "t2_1"})
	if err != nil {
		t.Fatal(err)
	}
	if unstartedVariant != "" {
		t.Fatal("expected variant to be nil")
	}
}

func TestNoBucketVal(t *testing.T) {
	experiment := NewSimpleExperiment(simpleConfig)
	result, err := experiment.Variant(map[string]string{"not_user_id": "t2_1"})
	expectedErr := "must specify user_id in call to variant for experiment test_experiment"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("expected error %s but was: %v", expectedErr, err)
	}
	if result != "" {
		t.Errorf("expected result to be empty but was %s", result)
	}

	experiment = NewSimpleExperiment(simpleConfig)
	result, err = experiment.Variant(map[string]string{"not_user_id": ""})
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
	experiment := NewSimpleExperiment(&config)
	variant, err := experiment.Variant(map[string]string{"user_id": "t2_2"})
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

	experiment1 := NewSimpleExperiment(simpleConfig)
	experiment2 := NewSimpleExperiment(&shuffleConfig)

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

func almostEqual(a, b, delta float64) (float64, bool) {
	if a == b {
		return 0.0, true
	}
	diff := math.Abs(a - b)
	if diff <= delta {
		return diff, true
	}
	round := roundTo(diff, 7)
	if round == 0.0 {
		return diff, true
	}
	return round, false
}

func roundTo(num float64, digits int) float64 {
	shift := math.Pow(10, float64(7)) // round to 7 digits
	return math.Round(num*shift) / shift
}
