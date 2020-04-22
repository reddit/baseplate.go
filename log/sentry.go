package log

import (
	"errors"
	"fmt"
	"io"
	"time"

	sentry "github.com/getsentry/sentry-go"
)

// DefaultSentryFlushTimeout is the timeout used to call sentry.Flush().
const DefaultSentryFlushTimeout = time.Second * 2

// ErrSentryFlushFailed is an error could be returned by the Closer returned by
// InitSentry, to indicate that the sentry flushing failed.
var ErrSentryFlushFailed = errors.New("log: sentry flushing failed")

// SentryConfig is the config to be passed into InitSentry.
//
// All fields are optional.
type SentryConfig struct {
	// The Sentry DSN to use.
	// If empty, SENTRY_DSN environment variable will be used instead.
	// If that's also empty, then all sentry operations will be nop.
	DSN string

	// SampleRate between 0 and 1, default is 1.
	SampleRate *float64

	// The name of your service.
	ServerName string

	// An environment string like "prod", "staging".
	Environment string

	// List of regexp strings that will be used to match against event's message
	// and if applicable, caught errors type and value.
	// If the match is found, then a whole event will be dropped.
	IgnoreErrors []string

	// FlushTimeout is the timeout to be used to call sentry.Flush when closing
	// the Closer returned by InitSentry.
	// If <=0, DefaultSentryFlushTimeout will be used.
	FlushTimeout time.Duration
}

// InitSentry initializes sentry reporting.
//
// The io.Closer returned calls sentry.Flush with SentryFlushTimeout.
// If it returns an error, that error is guaranteed to wrap
// ErrSentryFlushFailed.
//
// You can also just call sentry.Init,
// which provides even more customizations.
// This function is provided to do the customizations we care about the most,
// and to provide a Closer to be more consistent with other baseplate packages.
func InitSentry(cfg SentryConfig) (io.Closer, error) {
	var sampleRate float64 = 1
	if cfg.SampleRate != nil && *cfg.SampleRate >= 0 && *cfg.SampleRate <= 1 {
		sampleRate = *cfg.SampleRate
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:          cfg.DSN,
		SampleRate:   sampleRate,
		ServerName:   cfg.ServerName,
		Environment:  cfg.Environment,
		IgnoreErrors: cfg.IgnoreErrors,
	}); err != nil {
		return nil, err
	}
	return closer(cfg.FlushTimeout), nil
}

type closer time.Duration

func (c closer) Close() error {
	timeout := time.Duration(c)
	if timeout <= 0 {
		timeout = DefaultSentryFlushTimeout
	}
	if sentry.Flush(timeout) {
		return nil
	}
	return fmt.Errorf(
		"log: failed to flush sentry after %v: %w",
		timeout,
		ErrSentryFlushFailed,
	)
}
