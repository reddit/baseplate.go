package tracing

import (
	"github.com/opentracing/opentracing-go"
)

// StartSpanOptions is the additional options for baseplate spans.
type StartSpanOptions struct {
	OpenTracingOptions opentracing.StartSpanOptions

	Type SpanType
}

// Apply calls opt.Apply against sso.OpenTracingOptions.
//
// If opt also implements StartSpanOptions,
// it also calls opt.ApplyBP against sso.
func (sso *StartSpanOptions) Apply(opt opentracing.StartSpanOption) {
	opt.Apply(&sso.OpenTracingOptions)
	if o, ok := opt.(StartSpanOption); ok {
		o.ApplyBP(sso)
	}
}

// StartSpanOption defines additional options for baseplate spans.
type StartSpanOption interface {
	opentracing.StartSpanOption

	ApplyBP(*StartSpanOptions)
}

// nopOption implements opentracing.StartSpanOption with a nop Apply.
type nopOption struct{}

func (nopOption) Apply(*opentracing.StartSpanOptions) {}

// SpanTypeOption implements StartSpanOption to set the type of the span.
type SpanTypeOption struct {
	nopOption

	// NOTE: If the Type is SpanTypeClient,
	// the name of the span is expected (by metricsbp.CreateServerSpanHook)
	// to be in the format of "service.endpoint",
	// so that it can get the client and endpoint tags correctly.
	Type SpanType
}

// ApplyBP implements StartSpanOption.
func (s SpanTypeOption) ApplyBP(sso *StartSpanOptions) {
	sso.Type = s.Type
}

var (
	_ StartSpanOption = SpanTypeOption{}
)
