package metricsbp

import (
	"context"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"

	"github.com/reddit/baseplate.go/log"
)

// LogWrapperArgs defines the args used by LogWrapper.
type LogWrapperArgs struct {
	// The metrics path of the counter.
	//
	// Optional. If it's non-empty,
	// every time the generaged log.Wrapper is called, counter will be added by 1.
	Counter string

	// Additional tags to be applied to the metrics, optional.
	Tags Tags

	// Statsd to use.
	//
	// Optional. If this is nil, metricsbp.M will be used instead.
	Statsd *Statsd

	// The base log.Wrapper implementation.
	//
	// Optional. If this is nil,
	// then LogWrapper implementation will only emit metrics without logging.
	Wrapper log.Wrapper
}

// LogWrapper creates a log.Wrapper implementation with metrics emitting.
func LogWrapper(args LogWrapperArgs) log.Wrapper {
	var counter metrics.Counter
	if args.Counter != "" {
		counter = args.Statsd.Counter(args.Counter).With(args.Tags.AsStatsdTags()...)
	} else {
		counter = discard.NewCounter()
	}

	return func(ctx context.Context, msg string) {
		counter.Add(1)
		args.Wrapper.Log(ctx, msg)
	}
}
