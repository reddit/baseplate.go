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
)

type Document map[string]*ExperimentConfig

type Experiments struct {
	watcher *filewatcher.Result
}

func NewExperiments(ctx context.Context, path string, logger log.Wrapper) (*Experiments, error) {
	parser := func(r io.Reader) (interface{}, error) {
		var doc Document
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

type Bucket struct {
}

type Experiment interface {
	UniqueID(map[string]string) string
	Variant(args map[string]string) (string, error)
	LogBucketing() bool
}

func (e *Experiments) Variant(name string, args map[string]string, bucketingEventOverride bool) (string, error) {
	experiment, err := e.experiment(name)
	if err != nil {
		return "", err
	}
	return experiment.Variant(args)
}

func (e *Experiments) experiment(name string) (Experiment, error) {
	result := e.watcher.Get()
	doc, ok := result.(Document)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", doc)
	}
	experiment, ok := doc[name]
	if !ok {
		return nil, ErrorUnknownExperiment(name)
	}
	if isSimpleExperiment(experiment.Type) {
		return NewSimpleExperiment(experiment), nil
	}
	return nil, nil
}

type ParsedExperiment struct {
	ExperimentVersion int       `json:"experiment_version"`
	ShuffleVersion    string    `json:"shuffle_version"`
	BucketVal         string    `json:"bucket_val"`
	LogBucketing      bool      `json:"log_bucketing"`
	Variants          []Variant `json:"variants"`
	BucketSeed        string    `json:"bucket_seed"`
}

type ExperimentConfig struct {
	// ID is the experiment identifier and should be unique for each experiment.
	ID int `json:"id"`
	// Name is the experiment name and should be unique for each experiment.
	Name           string           `json:"name"`
	Owner          string           `json:"owner"`
	Enabled        *bool            `json:"enabled"`
	Version        string           `json:"version"`
	Type           string           `json:"type"`
	StartTimestamp int64            `json:"start_ts"`
	StopTimestamp  int64            `json:"stop_ts"`
	Experiment     ParsedExperiment `json:"experiment"`
}

type SimpleExperiment struct {
	id           int
	name         string
	seed         string
	numBuckets   int
	enabled      bool
	startTime    time.Time
	endTime      time.Time
	logBucketing bool
	bucketVal    string
	variantSet   VariantSet
}

// bucketVal == bucketing key, used to get the data
func NewSimpleExperiment(experiment *ExperimentConfig) *SimpleExperiment {
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
	return &SimpleExperiment{
		id:         experiment.ID,
		name:       experiment.Name,
		seed:       bucketSeed,
		bucketVal:  bucketVal,
		enabled:    enabled,
		startTime:  time.Unix(experiment.StartTimestamp, 0),
		endTime:    time.Unix(experiment.StopTimestamp, 0),
		numBuckets: 1000,
		variantSet: FromExperimentType(experiment.Type, experiment.Experiment.Variants, 0),
	}
}

func (e *SimpleExperiment) Variant(args map[string]string) (string, error) {
	if !e.isEnabled() {
		return "", nil
	}

	args = lowerArguments(args)
	if value, ok := args[e.bucketVal]; !ok || value == "" {
		return "", fmt.Errorf("must specify %s in call to variant for experiment %s", e.bucketVal, e.name)
	}

	// TODO: implement overrides

	// TODO: implement tareting

	bucket := e.calculateBucket(args[e.bucketVal])
	return e.variantSet.ChooseVariant(bucket), nil
}

func lowerArguments(args map[string]string) map[string]string {
	lowered := make(map[string]string, len(args))
	for key, value := range args {
		lowered[strings.ToLower(key)] = value
	}
	return lowered
}

func (e *SimpleExperiment) calculateBucket(bucketKey string) int {
	target := new(big.Int)
	bucket := new(big.Int)
	hashed := sha1.Sum([]byte(fmt.Sprintf("%s%s", e.seed, bucketKey)))
	target.SetBytes(hashed[:])
	bucket.Mod(target, big.NewInt(int64(e.numBuckets)))
	return int(bucket.Int64())
}

func (e *SimpleExperiment) UniqueID(bucketVals map[string]string) string {
	bucketVal, ok := bucketVals[e.bucketVal]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s", e.name, e.bucketVal, bucketVal)
}

func (e *SimpleExperiment) LogBucketing() bool {
	return e.logBucketing
}

func (e *SimpleExperiment) isEnabled() bool {
	now := time.Now()
	return e.enabled && !now.Before(e.startTime) && now.Before(e.endTime)
}

type Variant struct {
	Name string  `json:"name"`
	Size float64 `json:"size"`
}

type ErrorUnknownExperiment string

func (name ErrorUnknownExperiment) Error() string {
	return fmt.Sprintf("experiment with name %s unknown", name)
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
