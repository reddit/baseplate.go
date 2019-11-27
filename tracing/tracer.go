package tracing

import (
	"errors"

	zipkin "github.com/openzipkin/zipkin-go"
	"github.com/openzipkin/zipkin-go/reporter"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	xerrors "github.com/pkg/errors"
)

var zipkinReporter reporter.Reporter

// Tracer is exported to support start tracible server etc.
var Tracer *zipkin.Tracer

// ErrEndpointUndefined is an error could be returned by InitTracer.
var ErrEndpointUndefined = errors.New("endpoint to send spans to is undefined")

// InitTracer configure the global tracer.
func InitTracer(serviceName, addr string, sampleRate float64) error {
	if addr == "" {
		return ErrEndpointUndefined
	}

	zipkinReporter := zipkinhttp.NewReporter(addr)

	endpoint, err := zipkin.NewEndpoint(serviceName, "0.0.0.0:0")
	if err != nil {
		return xerrors.Wrap(err, "unable to create local endpoint")
	}

	sampler, err := zipkin.NewCountingSampler(sampleRate)
	if err != nil {
		return xerrors.Wrap(err, "unable to create sampler")
	}

	tracer, err := zipkin.NewTracer(
		zipkinReporter,
		zipkin.WithSampler(sampler),
		zipkin.WithLocalEndpoint(endpoint),
		zipkin.WithTraceID128Bit(false), // In baseplate we use 64 bit trace ids.
	)
	if err != nil {
		return xerrors.Wrap(err, "unable to create tracer")
	}

	Tracer = tracer
	return nil
}

// CloseZipkinReporter close the reporter started in init trace stage.
func CloseZipkinReporter() {
	zipkinReporter.Close()
}

// StartSpanFromParent creates a sub span from parent.
func StartSpanFromParent(optName string, parent zipkin.Span) zipkin.Span {
	var child zipkin.Span

	if parent != nil {
		child = Tracer.StartSpan(
			optName,
			zipkin.Parent(parent.Context()),
		)
	}

	return child
}
