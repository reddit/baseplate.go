package experiments

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/reddit/baseplate.go/filewatcher"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/timebp"
)

const targetAllOverride = `{"OVERRIDE": true}`

// Experiments offers access to the experiment framework with automatic refresh
// when there are change.
//
// This experiments client allows access to the experiments cached on disk by
// the experiment configuration fetcher daemon.  It will automatically reload
// the cache when changed.
type Experiments struct {
	watcher *filewatcher.Result
}

// NewExperiments returns a new instance of the experiments clients. The path
// points to the experiments file that will be parsed.
//
// Context should come with a timeout otherwise this might block forever, i.e.
// if the path never becomes available.
func NewExperiments(ctx context.Context, path string, logger log.Wrapper) (*Experiments, error) {
	parser := func(r io.Reader) (interface{}, error) {
		var doc document
		err := json.NewDecoder(r).Decode(&doc)
		if err != nil {
			return nil, err
		}
		return doc, nil
	}
	result, err := filewatcher.New(ctx, path, parser, logger)
	if err != nil {
		return nil, err
	}
	return &Experiments{
		watcher: result,
	}, nil
}

// Experiment is the interface for experiments.
type Experiment interface {
	UniqueID(map[string]string) string
	Variant(args map[string]interface{}) (string, error)
	LogBucketing() bool
}

// Variant determines the variant, if any, of this experiment is active.
//
// All arguments needed for bucketing, targeting, and variant overrides should
// be passed in as arguments. The parameter names are determined by the
// specific implementation of the Experiment interface.
//
// Returns the name of the enabled variant as a string if any variant is
// enabled. If no variant is enabled returns an empty string.
func (e *Experiments) Variant(name string, args map[string]interface{}, bucketingEventOverride bool) (string, error) {
	experiment, err := e.experiment(name)
	if err != nil {
		return "", err
	}
	return experiment.Variant(args)
}

func (e *Experiments) experiment(name string) (Experiment, error) {
	doc := e.watcher.Get().(document)
	experiment, ok := doc[name]
	if !ok {
		return nil, UnknownExperimentError(name)
	}
	if isSimpleExperiment(experiment.Type) {
		return NewSimpleExperiment(experiment)
	}
	// TODO handle this according to baseplate.py
	return nil, fmt.Errorf(
		"experiments.Experiments.Variant: unknown experiment %s",
		experiment.Type,
	)
}

// ParsedExperiment represents the experiment and configures the available
// variants.
type ParsedExperiment struct {
	ExperimentVersion int                          `json:"experiment_version"`
	ShuffleVersion    string                       `json:"shuffle_version"`
	BucketVal         string                       `json:"bucket_val"`
	LogBucketing      bool                         `json:"log_bucketing"`
	Variants          []Variant                    `json:"variants"`
	BucketSeed        string                       `json:"bucket_seed"`
	Targeting         json.RawMessage              `json:"targeting"`
	Overrides         []map[string]json.RawMessage `json:"overrides"`
}

type document map[string]*ExperimentConfig

// ExperimentConfig holds the information for the experiment plus additional
// data around the experiment.
type ExperimentConfig struct {
	// ID is the experiment identifier and should be unique for each experiment.
	ID int `json:"id"`
	// Name is the experiment name and should be unique for each experiment.
	Name string `json:"name"`
	// Owner is the group or individual that owns this experiment.
	Owner string `json:"owner"`
	// Enabled if set to false will disable the experiment and calls to Variant
	// will always returns an empty string.
	Enabled *bool `json:"enabled"`
	// Version is the string to identify the specific version of the
	// experiment.
	Version string `json:"version"`
	// Type specifies the type of experiment to run. If this value is not
	// recognized, the experiment will be considered disabled.
	Type string `json:"type"`
	// StartTimestamp is a float of seconds since the epoch of date and time
	// when you want the experiment to start. If an experiment has not been
	// started yet, it is considered disabled.
	StartTimestamp timebp.TimestampSecondF `json:"start_ts"`
	// StopTimestamp is a float of seconds since the epoch of date and time when
	// you want the experiment to stop. Once an experiment is stopped, it is
	// considered disabled.
	StopTimestamp timebp.TimestampSecondF `json:"stop_ts"`
	// Experiment is the specific experiment.
	Experiment ParsedExperiment `json:"experiment"`
}

// SimpleExperiment is a basic experiment choosing from a set of variants.
type SimpleExperiment struct {
	// id is the experiment identifier and should be unique.
	id int
	// name is a human-readable name of the experiment.
	name string
	// bucketSeed if provided, this provides the bucketSeed for determining which bucket a
	// variant request lands in. Providing a consistent bucket bucketSeed will ensure
	// a user is bucketed consistently. Calls to the variant method will return
	// consistent results for any given bucketSeed.
	bucketSeed string
	// numBuckets determines how many available buckets there are for bucketing
	// requests. This should match the numBuckets in the provided VariantSet.
	// The default value is 1000, which provides a potential variant
	// granularity of 0.1%.
	numBuckets int
	// enabled sets whether or not this experiment is enabled. disabling an
	// experiment means all variant calls will return an empty string.
	enabled bool
	// startTime determines when this experiment is due to start. Variant
	// requests prior to this time will return an empty string.
	startTime time.Time
	// endTime determines when this experiment is due to end. Variant requests
	// after this time will return an empty string.
	endTime time.Time
	// logBucketing determines whether bucketing events should be logged.
	logBucketing bool
	// bucketVal is a string used for shifting the deterministic bucketing
	// algorithm.  In most cases, this will be an Account's fullname.
	bucketVal string
	// variantSet contains a set of experimental variants as well as their
	// distributions. It is used by experiments to track which bucket a variant
	// is assigned to.
	variantSet VariantSet
	// targeting allows to target users with multiple parameters supporting
	// both AND and OR based logical grouping.
	targeting Targeting
	// overrides if matched allow to force a particular variant.
	overrides []map[string]Targeting
}

// NewSimpleExperiment returns a new instance of SimpleExperiment. Default
// values if not otherwise provided by the ExperimentConfig will be assumed.
func NewSimpleExperiment(experiment *ExperimentConfig) (*SimpleExperiment, error) {
	shuffleVersion := experiment.Experiment.ShuffleVersion
	if shuffleVersion == "" {
		shuffleVersion = "None"
	}
	bucketVal := experiment.Experiment.BucketVal
	if bucketVal == "" {
		bucketVal = "user_id"
	}
	enabled := true
	if experiment.Enabled != nil {
		enabled = *experiment.Enabled
	}
	bucketSeed := experiment.Experiment.BucketSeed
	if experiment.Experiment.BucketSeed == "" {
		bucketSeed = fmt.Sprintf("%d.%s.%s", experiment.ID, experiment.Name, shuffleVersion)
	}
	variantSet, err := FromExperimentType(experiment.Type, experiment.Experiment.Variants, 0)
	if err != nil {
		return nil, err
	}

	targetingConfig := experiment.Experiment.Targeting
	if len(targetingConfig) == 0 {
		targetingConfig = []byte(targetAllOverride)
	}
	targeting, err := NewTargeting(targetingConfig)
	if err != nil {
		return nil, err
	}
	overrides := make([]map[string]Targeting, len(experiment.Experiment.Overrides))
	for i, override := range experiment.Experiment.Overrides {
		for variant, overrideConfig := range override {
			override, err := NewTargeting(overrideConfig)
			if err != nil {
				return nil, err
			}
			if overrides[i] == nil {
				overrides[i] = make(map[string]Targeting)
			}
			overrides[i][variant] = override
		}
	}
	return &SimpleExperiment{
		id:         experiment.ID,
		name:       experiment.Name,
		bucketSeed: bucketSeed,
		bucketVal:  bucketVal,
		enabled:    enabled,
		startTime:  experiment.StartTimestamp.ToTime(),
		endTime:    experiment.StopTimestamp.ToTime(),
		numBuckets: 1000,
		variantSet: variantSet,
		targeting:  targeting,
		overrides:  overrides,
	}, nil
}

// Variant determines the variant, if any, is active. Bucket calculation is
// determined based on the bucketVal.
func (e *SimpleExperiment) Variant(args map[string]interface{}) (string, error) {
	if !e.isEnabled() {
		return "", nil
	}
	args = lowerArguments(args)
	if value, ok := args[e.bucketVal]; !ok || value == "" {
		return "", fmt.Errorf(
			"experiment.SimpleExperiment.Variant: must specify %s in call to variant for experiment %s",
			e.bucketVal,
			e.name,
		)
	}
	for _, override := range e.overrides {
		for variant, targeting := range override {
			if targeting.Evaluate(args) {
				return variant, nil
			}
		}
	}
	if !e.targeting.Evaluate(args) {
		return "", nil
	}
	bucketVal, ok := args[e.bucketVal].(string)
	if !ok {
		return "", fmt.Errorf(
			"experiment.SimpleExperiment.Variant: expected bucket val to be a string, actual: %T",
			args[e.bucketVal],
		)
	}

	bucket := e.calculateBucket(bucketVal)
	return e.variantSet.ChooseVariant(bucket), nil
}

func lowerArguments(args map[string]interface{}) map[string]interface{} {
	lowered := make(map[string]interface{}, len(args))
	for key, value := range args {
		lowered[strings.ToLower(key)] = value
	}
	return lowered
}

func (e *SimpleExperiment) calculateBucket(bucketKey string) int {
	target := new(big.Int)
	bucket := new(big.Int)
	hashed := sha1.Sum([]byte(e.bucketSeed + bucketKey))
	target.SetBytes(hashed[:])
	bucket.Mod(target, big.NewInt(int64(e.numBuckets)))
	return int(bucket.Int64())
}

// UniqueID returns a unique ID for the experiment.
func (e *SimpleExperiment) UniqueID(bucketVals map[string]string) string {
	bucketVal, ok := bucketVals[e.bucketVal]
	if !ok {
		return ""
	}
	return strings.Join([]string{e.name, e.bucketVal, bucketVal}, ":")
}

// LogBucketing returns whether or not this experiment should log bucketing events.
func (e *SimpleExperiment) LogBucketing() bool {
	return e.logBucketing
}

func (e *SimpleExperiment) isEnabled() bool {
	now := time.Now()
	return e.enabled && !now.Before(e.startTime) && now.Before(e.endTime)
}

// Variant is a single variant that belongs to a set of variants and determines
// a bucket by name and size. Either size is set or range start and range end.
type Variant struct {
	Name       string  `json:"name"`
	Size       float64 `json:"size"`
	RangeStart float64 `json:"range_start"`
	RangeEnd   float64 `json:"range_end"`
}

// UnknownExperimentError is returned if the configured experiment is not
// known.
type UnknownExperimentError string

func (name UnknownExperimentError) Error() string {
	return fmt.Sprintf("experiments: experiment with name %s unknown", name)
}

func isSimpleExperiment(experimentType string) bool {
	switch experimentType {
	case "single_variant":
	case "multi_variant":
	case "feature_rollout":
	case "range_variant":
		return true
	}
	return false
}
