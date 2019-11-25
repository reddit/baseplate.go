package tracing

import (
	"fmt"

	"github.com/opentracing/opentracing-go"
	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"
	zipkin "github.com/openzipkin/zipkin-go"
	"github.com/openzipkin/zipkin-go/reporter"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/pkg/errors"
)

var zipkinReporter reporter.Reporter

// Tracer is exported to support start tracible server etc
var Tracer = opentracing.GlobalTracer()

// InitTracer configure the global tracer
func InitTracer(serviceName, addr string, sampleRate float64,
) error {
	if addr == "" {
		return errors.New("Endpoint to send spans to is undefined")
	}

	zipkinReporter := zipkinhttp.NewReporter(addr)

	endpoint, err := zipkin.NewEndpoint(serviceName, "0.0.0.0:0")
	if err != nil {
		return errors.Wrap(err, "Unable to create local endpoint")
	}

	sampler, err := zipkin.NewCountingSampler(sampleRate)
	if err != nil {
		return errors.Wrap(err, "Unable to create sampler")
	}

	nativeTracer, err := zipkin.NewTracer(
		zipkinReporter,
		zipkin.WithSampler(sampler),
		zipkin.WithLocalEndpoint(endpoint),
	)

	tracer := zipkinot.Wrap(nativeTracer)
	opentracing.SetGlobalTracer(tracer)

	return nil
}

// CloseZipkinReporter close the reporter started in init trace stage
func CloseZipkinReporter() {
	zipkinReporter.Close()
}

// StartSpanFromParent creates a sub span from parent
func StartSpanFromParent(optName string, parent opentracing.Span) opentracing.Span {
	var child opentracing.Span

	if parent != nil {
		child = Tracer.StartSpan(
			optName,
			opentracing.ChildOf(parent.Context()),
		)
	}

	return child
}

// EndSpan finish the span with option messages
func EndSpan(span opentracing.Span, opts ...interface{}) {
	if span == nil {
		return
	}

	if len(opts) > 0 {
		if len(opts)%2 != 0 {
			opts = append(opts, "Missing Value")
		}

		values := make([]opentracing.LogRecord, len(opts)/2)
		for i := 0; i < len(opts); i += 2 {
			ld := &opentracing.LogData{
				Event:   fmt.Sprintf("%v", opts[i]),
				Payload: opts[i+1],
			}
			values = append(values, ld.ToLogRecord())
		}

		span.FinishWithOptions(opentracing.FinishOptions{LogRecords: values})
	} else {
		span.Finish()
	}
}
