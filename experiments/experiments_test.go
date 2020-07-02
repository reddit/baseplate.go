package experiments

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/timebp"
)

var simpleConfig = &ExperimentConfig{
	ID:             1,
	Name:           "test_experiment",
	Owner:          "test",
	Type:           "single_variant",
	Version:        "1",
	StartTimestamp: timebp.TimestampSecondF(time.Now().Add(-30 * 24 * time.Hour)),
	StopTimestamp:  timebp.TimestampSecondF(time.Now().Add(30 * 24 * time.Hour)),
	Enabled:        func() *bool { b := true; return &b }(),
	Experiment: Experiment{
		BucketSeed: "some new seed",
		Variants: []Variant{
			{
				Name: "variant_1",
				Size: 0.1,
			},
			{
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
		shuffleVersion int
		expectedBucket int
	}{
		{
			config: &ExperimentConfig{
				ID:   1,
				Name: "test_experiment",
				Experiment: Experiment{
					ShuffleVersion: 0,
					BucketSeed:     "some new seed",
					Variants: []Variant{
						{
							Name: "variant_1",
							Size: 0.1,
						},
						{
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
		StartTimestamp: timebp.TimestampSecondF(time.Now().Add(-30 * 24 * time.Hour)),
		StopTimestamp:  timebp.TimestampSecondF(time.Now().Add(30 * 24 * time.Hour)),
		Enabled:        func() *bool { b := true; return &b }(),
		Experiment: Experiment{
			Variants: []Variant{
				{
					Name: "variant_1",
					Size: 0.1,
				},
				{
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

	users := experiment.numBuckets * 500
	names := make([]string, 0, users)
	for i := 0; i < users; i++ {
		names = append(names, fmt.Sprintf("t2_%d", i))
	}

	counter := make(map[int]int, experiment.numBuckets)
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
		d, ok := almostEqual(percentEqual, 1, 0.15)
		if !ok {
			t.Errorf("bucket %d: %f, expected %d, actual %d", i, d, expected, actual)
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
		StartTimestamp: timebp.TimestampSecondF(time.Now().Add(-30 * 24 * time.Hour)),
		StopTimestamp:  timebp.TimestampSecondF(time.Now().Add(30 * 24 * time.Hour)),
		Enabled:        func() *bool { b := true; return &b }(),
		Experiment: Experiment{
			BucketSeed: "some new seed",
			Variants: []Variant{
				{
					Name: "variant_1",
					Size: 0.1,
				},
				{
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

	users := experiment.numBuckets * 500
	names := make([]string, 0, users)
	for i := 0; i < users; i++ {
		names = append(names, fmt.Sprintf("t2_%d", i))
	}

	counter := make(map[int]int, experiment.numBuckets)
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
		d, ok := almostEqual(percentEqual, 1, 0.15)
		if !ok {
			t.Errorf("bucket %d: %f, expected %d, actual %d", i, d, expected, actual)
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

func TestVariantExplicitNil(t *testing.T) {
	validExperiment, err := NewSimpleExperiment(simpleConfig)
	if err != nil {
		t.Fatal(err)
	}

	// Explicit nil user_id is caused by empty loid.
	// We just treat them the same as empty string.
	_, err = validExperiment.Variant(map[string]interface{}{"user_id": nil})
	if err == nil {
		t.Error("Expected error for explicit nil user_id, got nil error.")
	}
	if !errors.As(err, new(MissingBucketKeyError)) {
		t.Errorf("Expected MissingBucketKeyError for explicit nil user_id, got %v", err)
	}
}

func TestNoBucketVal(t *testing.T) {
	experiment, err := NewSimpleExperiment(simpleConfig)
	if err != nil {
		t.Fatal(err)
	}
	result, err := experiment.Variant(map[string]interface{}{"not_user_id": "t2_1"})
	if err == nil {
		t.Error("Expected error for missing user_id, got nil error.")
	}
	if !errors.As(err, new(MissingBucketKeyError)) {
		t.Errorf("Expected MissingBucketKeyError for missing user_id, got %v", err)
	}
	if result != "" {
		t.Errorf("expected result to be empty but was %s", result)
	}

	experiment, err = NewSimpleExperiment(simpleConfig)
	if err != nil {
		t.Fatal(err)
	}
	result, err = experiment.Variant(map[string]interface{}{"not_user_id": ""})
	if err == nil {
		t.Error("Expected error for missing user_id, got nil error.")
	}
	if !errors.As(err, new(MissingBucketKeyError)) {
		t.Errorf("Expected MissingBucketKeyError for missing user_id, got %v", err)
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
	shuffleConfig.Experiment.ShuffleVersion = 2

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
		StartTimestamp: timebp.TimestampSecondF(time.Now().Add(-30 * 24 * time.Hour)),
		StopTimestamp:  timebp.TimestampSecondF(time.Now().Add(30 * 24 * time.Hour)),
		Enabled:        func() *bool { b := true; return &b }(),
		Experiment: Experiment{
			Variants: []Variant{
				{
					Name: "variant_1",
					Size: 0.1,
				},
				{
					Name: "variant_2",
					Size: 0.1,
				},
			},
			ExperimentVersion: 1,
			Overrides: []map[string]json.RawMessage{
				{
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
	if variant != "variant_1" && variant != "variant_2" { // <-bug!!! t2_123 actually falls into "no treatment" bucket
		t.Errorf("expected %s or %s, actual: %s", "variant_1", "variant_2", variant)
	}
}

func TestTreatmentsDistribution(t *testing.T) {
	t.Parallel()
	userIDs := make([]string, 100)
	for i := 0; i < len(userIDs); i++ {
		userIDs[i] = fmt.Sprintf("t2_%02d", i)
	}

	t.Run("single_variant type", func(t *testing.T) {
		t.Parallel()
		config := makeTestConfig(
			"single_variant",
			Variant{Name: "variant_1", Size: 0.1},
			Variant{Name: "variant_2", Size: 0.2},
		)
		experiment, err := NewSimpleExperiment(config)
		if err != nil {
			t.Fatal(err)
		}

		buckets := map[string]int{
			"variant_1": 0,
			"variant_2": 0,
			"":          0,
		}
		for _, userID := range userIDs {
			variant, err := experiment.Variant(map[string]interface{}{"user_id": userID})
			if err != nil {
				t.Fatal(err)
			}
			buckets[variant]++
		}

		if buckets["variant_1"] != 0 { // <-bug!!! should be ~10
			t.Errorf("expected %d, actual: %d", 0, buckets["variant_1"])
		}
		if buckets["variant_2"] != 100 { // <-bug!!! should be ~20
			t.Errorf("expected %d, actual: %d", 100, buckets["variant_2"])
		}
		if buckets[""] != 0 { // <-bug!!! should be ~70
			t.Errorf("expected %d, actual: %d", 0, buckets[""])
		}
	})

	t.Run("multi_variant type", func(t *testing.T) {
		t.Parallel()
		config := makeTestConfig(
			"multi_variant",
			Variant{Name: "variant_1", Size: 0.1},
			Variant{Name: "variant_2", Size: 0.2},
			Variant{Name: "variant_3", Size: 0.3},
		)
		experiment, err := NewSimpleExperiment(config)
		if err != nil {
			t.Fatal(err)
		}

		buckets := map[string]int{
			"variant_1": 0,
			"variant_2": 0,
			"variant_3": 0,
			"":          0,
		}
		for _, userID := range userIDs {
			variant, err := experiment.Variant(map[string]interface{}{"user_id": userID})
			if err != nil {
				t.Fatal(err)
			}
			buckets[variant]++
		}

		if buckets["variant_1"] != 0 { // <-bug!!! should be ~10
			t.Errorf("expected %d, actual: %d", 0, buckets["variant_1"])
		}
		if buckets["variant_2"] != 0 { // <-bug!!! should be ~20
			t.Errorf("expected %d, actual: %d", 0, buckets["variant_2"])
		}
		if buckets["variant_3"] != 0 { // <-bug!!! should be ~30
			t.Errorf("expected %d, actual: %d", 0, buckets["variant_3"])
		}
		if buckets[""] != 100 { // <-bug!!! should be ~40
			t.Errorf("expected %d, actual: %d", 100, buckets[""])
		}
	})

	t.Run("feature_rollout type", func(t *testing.T) {
		t.Parallel()
		config := makeTestConfig(
			"feature_rollout",
			Variant{Name: "variant_1", Size: 0.1},
		)
		experiment, err := NewSimpleExperiment(config)
		if err != nil {
			t.Fatal(err)
		}

		buckets := map[string]int{
			"variant_1": 0,
			"":          0,
		}
		for _, userID := range userIDs {
			variant, err := experiment.Variant(map[string]interface{}{"user_id": userID})
			if err != nil {
				t.Fatal(err)
			}
			buckets[variant]++
		}

		if buckets["variant_1"] != 0 { // <-bug!!! should be ~10
			t.Errorf("expected %d, actual: %d", 0, buckets["variant_1"])
		}
		if buckets[""] != 100 { // <-bug!!! should be ~90
			t.Errorf("expected %d, actual: %d", 100, buckets[""])
		}
	})

	t.Run("range_variant type", func(t *testing.T) {
		t.Parallel()
		config := makeTestConfig(
			"range_variant",
			Variant{Name: "variant_1", RangeStart: 0.1, RangeEnd: 0.2},
			Variant{Name: "variant_2", RangeStart: 0.4, RangeEnd: 0.6},
		)
		experiment, err := NewSimpleExperiment(config)
		if err != nil {
			t.Fatal(err)
		}

		buckets := map[string]int{
			"variant_1": 0,
			"variant_2": 0,
			"":          0,
		}
		for _, userID := range userIDs {
			variant, err := experiment.Variant(map[string]interface{}{"user_id": userID})
			if err != nil {
				t.Fatal(err)
			}
			buckets[variant]++
		}

		if buckets["variant_1"] != 0 { // <-bug!!! should be ~10
			t.Errorf("expected %d, actual: %d", 0, buckets["variant_1"])
		}
		if buckets["variant_2"] != 0 { // <-bug!!! should be ~20
			t.Errorf("expected %d, actual: %d", 0, buckets["variant_2"])
		}
		if buckets[""] != 100 { // <-bug!!! should be ~70
			t.Errorf("expected %d, actual: %d", 100, buckets[""])
		}
	})
}

func makeTestConfig(experimentType string, variants ...Variant) *ExperimentConfig {
	return &ExperimentConfig{
		ID:             1,
		Name:           "test_experiment",
		Owner:          "test",
		Type:           experimentType,
		Version:        "1",
		StartTimestamp: timebp.TimestampSecondF(time.Now().Add(-30 * 24 * time.Hour)),
		StopTimestamp:  timebp.TimestampSecondF(time.Now().Add(30 * 24 * time.Hour)),
		Enabled:        func() *bool { b := true; return &b }(),
		Experiment: Experiment{
			Variants:          variants,
			ExperimentVersion: 1,
		},
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
