package log

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	sentry "github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
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
	DSN string `yaml:"dsn"`

	// SampleRate between 0 and 1, default is 1.
	SampleRate *float64 `yaml:"sampleRate"`

	// The name of your service.
	//
	// By default sentry extracts hostname reported by the kernel for this field.
	// Set it to non-empty to override that
	// (and hostname tag will be used to report hostname automatically).
	ServerName string `yaml:"serverName"`

	// An environment string like "prod", "staging".
	Environment string `yaml:"environment"`

	// List of regexp strings that will be used to match against event's message
	// and if applicable, caught errors type and value.
	// If the match is found, then a whole event will be dropped.
	IgnoreErrors []string `yaml:"ignoreErrors"`

	// FlushTimeout is the timeout to be used to call sentry.Flush when closing
	// the Closer returned by InitSentry.
	// If <=0, DefaultSentryFlushTimeout will be used.
	FlushTimeout time.Duration `yaml:"flushTimeout"`

	// BeforeSend is a callback modifier before emitting an event to Sentry.
	BeforeSend func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event `yaml:"-"`
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
//
// When cfg.ServerName is non-empty,
// this function also sets hostname tag to the value reported by the kernel
// (when cfg.ServerName is empty server_name tag will be the hostname).
func InitSentry(cfg SentryConfig) (io.Closer, error) {
	var sampleRate float64 = 1
	if cfg.SampleRate != nil && *cfg.SampleRate >= 0 && *cfg.SampleRate <= 1 {
		sampleRate = *cfg.SampleRate
	}

	// Improve legibility of Sentry errors by using the error message as header
	// instead of the error type and marking stack trace frame from
	// baseplate.go as not in-app.
	const (
		base   = "github.com/reddit/baseplate.go"
		prefix = base + "/"
	)
	beforeSend := func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
		for i, exception := range event.Exception {
			// swap source location and of error type as title in issue list
			// https://github.com/getsentry/sentry-go/issues/307#issuecomment-737871530
			event.Exception[i].Type, event.Exception[i].Value = event.Exception[i].Value, event.Exception[i].Type
			if exception.Stacktrace != nil {
				for j, frame := range exception.Stacktrace.Frames {
					if frame.Module == base || strings.HasPrefix(frame.Module, prefix) {
						event.Exception[i].Stacktrace.Frames[j].InApp = false
					}
				}
			}
		}
		if cfg.BeforeSend != nil {
			return cfg.BeforeSend(event, hint)
		}
		return event
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:          cfg.DSN,
		SampleRate:   sampleRate,
		ServerName:   cfg.ServerName,
		Environment:  cfg.Environment,
		IgnoreErrors: cfg.IgnoreErrors,
		BeforeSend:   beforeSend,
	}); err != nil {
		return nil, err
	}
	if cfg.ServerName != "" {
		if hostname, err := os.Hostname(); err == nil {
			sentry.CurrentHub().ConfigureScope(func(scope *sentry.Scope) {
				scope.SetTag("hostname", hostname)
			})
		}
	}
	if Version != "" {
		sentry.CurrentHub().ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag("version", Version)
		})
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

// ErrorWithSentry logs a message with some additional context,
// then sends the error to Sentry.
//
// The variadic key-value pairs are treated as they are in With.
// and will also be sent to sentry.
// Note that zap.Field is not supported here and will be ignored while sending
// to sentry (but they will be logged to error log).
//
// If a sentry hub is attached to the context object passed in
// (it will be if the context object is from baseplate hooked request context),
// that hub will be used to do the reporting.
// Otherwise the global sentry hub will be used instead.
func ErrorWithSentry(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	if len(keysAndValues) > 0 {
		hub = hub.Clone()
		hub.ConfigureScope(func(scope *sentry.Scope) {
			if extractKeyValuePairs(keysAndValues, scope.SetTag) {
				Errorw(
					"Dangling key in ErrorWithSentry",
					"keysAndValues", keysAndValues,
				)
			}
		})
	}

	keysAndValues = append(keysAndValues, "err", err)
	C(ctx).Errorw(msg, keysAndValues...)
	hub.CaptureException(err)
}

func extractKeyValuePairs(keysAndValues []interface{}, f func(key, value string)) (danglingKey bool) {
	for i := 0; i < len(keysAndValues); i++ {
		if _, ok := keysAndValues[i].(zapcore.Field); ok {
			// We don't support this type right now,
			// and they don't appear in pairs. just ignore them.
			continue
		}

		if i == len(keysAndValues)-1 {
			// this is a dangling key.
			return true
		}

		// In zap logger they are handled differently.
		// Here we just use fmt.Sprintf("%v") to keep things simple.
		key := fmt.Sprintf("%v", keysAndValues[i])
		// extra i++ needed here because we need to consume the pair.
		i++
		value := fmt.Sprintf("%v", keysAndValues[i])
		f(key, value)
	}
	return false
}
