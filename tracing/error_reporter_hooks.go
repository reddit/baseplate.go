package tracing

// ErrorReporterCreateServerSpanHook registers each Server Span with an
// ErrorReporterSpanHook that will publish errors sent to OnPreStop to Sentry.
type ErrorReporterCreateServerSpanHook struct{}

// OnCreateServerSpan registers SentrySpanHook on a Server Span.
func (h ErrorReporterCreateServerSpanHook) OnCreateServerSpan(span *Span) error {
	span.AddHooks(errorReporterSpanHook{})
	return nil
}

// errorReporterSpanHook publishes errors sent to OnPreStop to Sentry.
//
// This Hook is only registered to ServerSpans.
type errorReporterSpanHook struct{}

// OnPostStart is a no-op
func (h errorReporterSpanHook) OnPostStart(span *Span) error {
	return nil
}

// OnPreStop logs a message and sends err to Sentry if err is non-nil.
func (h errorReporterSpanHook) OnPreStop(span *Span, err error) error {
	if err != nil {
		span.getHub().CaptureException(err)
	}
	return nil
}

var (
	_ CreateServerSpanHook = ErrorReporterCreateServerSpanHook{}
	_ StartStopSpanHook    = errorReporterSpanHook{}
)
