package tracing

import (
	"github.com/getsentry/raven-go"
)

// ErrorReporterBaseplateHook registers each Server Span with an
// ErrorReporterSpanHook that will publish errors sent to OnEnd to Sentry.
type ErrorReporterBaseplateHook struct{}

// OnServerSpanCreate registers SentrySpanHook on a Server Span.
func (h ErrorReporterBaseplateHook) OnServerSpanCreate(span *Span) error {
	span.RegisterHook(ErrorReporterSpanHook{})
	return nil
}

// ErrorReporterSpanHook publishes errors sent to OnEnd to Sentry.
type ErrorReporterSpanHook struct{}

// OnCreateChild is a nop.
func (h ErrorReporterSpanHook) OnCreateChild(child *Span) error {
	return nil
}

// OnStart is a nop
func (h ErrorReporterSpanHook) OnStart(span *Span) error {
	return nil
}

// OnEnd logs a message and sends err to Sentry if err is non-nil.
func (h ErrorReporterSpanHook) OnEnd(span *Span, err error) error {
	if err != nil {
		raven.CaptureError(err, nil)
	}
	return nil
}

// OnSetTag is a nop
func (h ErrorReporterSpanHook) OnSetTag(span *Span, key string, value interface{}) error {
	return nil
}

// OnAddCounter is a nop
func (h ErrorReporterSpanHook) OnAddCounter(span *Span, key string, delta float64) error {
	return nil
}

var (
	_ BaseplateHook = ErrorReporterBaseplateHook{}
	_ SpanHook      = ErrorReporterSpanHook{}
)
