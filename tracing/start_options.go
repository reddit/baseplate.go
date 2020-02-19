package tracing

import (
	opentracing "github.com/opentracing/opentracing-go"
)

// StartSpanOptions is the additional options for baseplate spans.
type StartSpanOptions struct {
	OpenTracingOptions opentracing.StartSpanOptions

	Type          SpanType
	ComponentName string
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

	Type SpanType
}

// ApplyBP implements StartSpanOption.
func (s SpanTypeOption) ApplyBP(sso *StartSpanOptions) {
	sso.Type = s.Type
}

// LocalComponentOption implements StartSpanOption to set the type to local and
// also sets the local component name.
type LocalComponentOption struct {
	nopOption

	Name string
}

// ApplyBP implements StartSpanOption.
func (l LocalComponentOption) ApplyBP(sso *StartSpanOptions) {
	sso.Type = SpanTypeLocal
	sso.ComponentName = l.Name
}

var (
	_ StartSpanOption = SpanTypeOption{}
	_ StartSpanOption = LocalComponentOption{}
)
