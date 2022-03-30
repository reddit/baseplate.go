package thriftbp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/iobp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/prometheusbp"
	"github.com/reddit/baseplate.go/randbp"
	"github.com/reddit/baseplate.go/tracing"
	"github.com/reddit/baseplate.go/transport"
)

var (
	_ thrift.ProcessorMiddleware = ExtractDeadlineBudget
	_ thrift.ProcessorMiddleware = AbandonCanceledRequests
)

// DefaultProcessorMiddlewaresArgs are the args to be passed into
// BaseplateDefaultProcessorMiddlewares function to create default processor
// middlewares.
type DefaultProcessorMiddlewaresArgs struct {
	// Suppress some of the errors returned by the server before sending them to
	// the server span.
	//
	// Based on Baseplate spec, the errors defined in your thrift IDL are not
	// treated as errors, and should be suppressed here. So in most cases that's
	// what the service developer should implement as the Suppressor here.
	//
	// Note that this suppressor only affects the errors send to the span. It
	// won't affect the errors returned to the client.
	//
	// This is optional. If it's not set IDLExceptionSuppressor will be used.
	ErrorSpanSuppressor errorsbp.Suppressor

	// Report the payload size metrics with this sample rate.
	//
	// This is optional. If it's not set none of the requests will be sampled.
	ReportPayloadSizeMetricsSampleRate float64

	// The edge context implementation. Optional.
	//
	// If it's not set, the global one from ecinterface.Get will be used instead.
	EdgeContextImpl ecinterface.Interface
}

// BaseplateDefaultProcessorMiddlewares returns the default processor
// middlewares that should be used by a baseplate Thrift service.
//
// Currently they are (in order):
//
// 1. ExtractDeadlineBudget
//
// 2. InjectServerSpan
//
// 3. InjectEdgeContext
//
// 4. AbandonCanceledRequests
//
// 5. ReportPayloadSizeMetrics
//
// 6. PrometheusServerMiddleware
//
// 7. RecoverPanic
func BaseplateDefaultProcessorMiddlewares(args DefaultProcessorMiddlewaresArgs) []thrift.ProcessorMiddleware {
	return []thrift.ProcessorMiddleware{
		ExtractDeadlineBudget,
		InjectServerSpan(args.ErrorSpanSuppressor),
		InjectEdgeContext(args.EdgeContextImpl),
		AbandonCanceledRequests,
		ReportPayloadSizeMetrics(args.ReportPayloadSizeMetricsSampleRate),
		PrometheusServerMiddleware,
		RecoverPanic,
	}
}

// StartSpanFromThriftContext creates a server span from thrift context object.
//
// This span would usually be used as the span of the whole thrift endpoint
// handler, and the parent of the child-spans.
//
// Caller should pass in the context object they got from thrift library,
// which would have all the required headers already injected.
//
// Please note that "Sampled" header is default to false according to baseplate
// spec, so if the context object doesn't have headers injected correctly,
// this span (and all its child-spans) will never be sampled,
// unless debug flag was set explicitly later.
//
// If any of the tracing related thrift header is present but malformed,
// it will be ignored.
// The error will also be logged if InitGlobalTracer was last called with a
// non-nil logger.
// Absent tracing related headers are always silently ignored.
func StartSpanFromThriftContext(ctx context.Context, name string) (context.Context, *tracing.Span) {
	var headers tracing.Headers
	var sampled bool

	if str, ok := Header(ctx, transport.HeaderTracingTrace); ok {
		headers.TraceID = str
	}
	if str, ok := Header(ctx, transport.HeaderTracingSpan); ok {
		headers.SpanID = str
	}
	if str, ok := Header(ctx, transport.HeaderTracingFlags); ok {
		headers.Flags = str
	}
	if str, ok := Header(ctx, transport.HeaderTracingSampled); ok {
		sampled = str == transport.HeaderTracingSampledTrue
		headers.Sampled = &sampled
	}

	return tracing.StartSpanFromHeaders(ctx, name, headers)
}

// InjectServerSpan implements thrift.ProcessorMiddleware and injects a server
// span into the `next` context.
//
// Starts the server span before calling the `next` TProcessorFunction and stops
// the span after it finishes.
// If the function returns an error that's not suppressed by the suppressor,
// that will be passed to span.Stop.
//
// Please note that if suppressor passed in is nil,
// it will be changed to IDLExceptionSuppressor instead.
// Please use errorsbp.SuppressNone explicitly instead if that's what's wanted.
//
// If "User-Agent" (HeaderUserAgent) THeader is set,
// the created server span will also have
// "peer.service" (tracing.TagKeyPeerService) tag set to its value.
//
// Note, the span will be created according to tracing related headers already
// being set on the context object.
// These should be automatically injected by your thrift.TSimpleServer.
func InjectServerSpan(suppressor errorsbp.Suppressor) thrift.ProcessorMiddleware {
	if suppressor == nil {
		suppressor = IDLExceptionSuppressor
	}
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (success bool, err thrift.TException) {
				ctx, span := StartSpanFromThriftContext(ctx, name)
				if userAgent, ok := Header(ctx, transport.HeaderUserAgent); ok {
					span.SetTag(tracing.TagKeyPeerService, userAgent)
				}
				defer func() {
					span.FinishWithOptions(tracing.FinishOptions{
						Ctx: ctx,
						Err: suppressor.Wrap(err),
					}.Convert())
				}()

				return next.Process(ctx, seqID, in, out)
			},
		}
	}
}

// InitializeEdgeContext sets an edge request context created from the Thrift
// headers set on the context onto the context and configures Thrift to forward
// the edge requent context header on any Thrift calls made by the server.
func InitializeEdgeContext(ctx context.Context, impl ecinterface.Interface) context.Context {
	header, ok := Header(ctx, transport.HeaderEdgeRequest)
	if !ok {
		return ctx
	}

	ctx, err := impl.HeaderToContext(ctx, header)
	if err != nil {
		log.Error("Error while parsing EdgeRequestContext: " + err.Error())
	}
	return ctx
}

// InjectEdgeContext returns a ProcessorMiddleware that injects an edge request
// context created from the Thrift headers set on the context into the `next`
// thrift.TProcessorFunction.
//
// Note, this depends on the edge context headers already being set on the
// context object.  These should be automatically injected by your
// thrift.TSimpleServer.
func InjectEdgeContext(impl ecinterface.Interface) thrift.ProcessorMiddleware {
	if impl == nil {
		impl = ecinterface.Get()
	}
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				ctx = InitializeEdgeContext(ctx, impl)
				return next.Process(ctx, seqID, in, out)
			},
		}
	}
}

// ExtractDeadlineBudget is the server middleware implementing Phase 1 of
// Baseplate deadline propagation.
//
// It only sets the timeout if the passed in deadline is at least 1ms.
func ExtractDeadlineBudget(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
	return thrift.WrappedTProcessorFunction{
		Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
			if s, ok := Header(ctx, transport.HeaderDeadlineBudget); ok {
				v, err := strconv.ParseInt(s, 10, 64)
				if err == nil && v >= 1 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, time.Millisecond*time.Duration(v))
					defer cancel()
				}
			}
			return next.Process(ctx, seqID, in, out)
		},
	}
}

// AbandonCanceledRequests transforms context.Canceled errors into
// thrift.ErrAbandonRequest errors.
//
// When using thrift compiler version >4db7a0a, the context object will be
// canceled after the client closes the connection, and returning
// thrift.ErrAbandonRequest as the error helps the server to not try to write
// the error back to the client, but close the connection directly.
func AbandonCanceledRequests(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
	return thrift.WrappedTProcessorFunction{
		Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
			ok, err := next.Process(ctx, seqID, in, out)
			if errors.Is(err, context.Canceled) {
				err = thrift.WrapTException(thrift.ErrAbandonRequest)
			}
			return ok, err
		},
	}
}

// ReportPayloadSizeMetrics returns a ProcessorMiddleware that reports metrics
// (histograms) of request and response payload sizes in bytes.
//
// This middleware only works on sampled requests with the given sample rate,
// but the histograms it reports are overriding global histogram sample rate
// with 100% sample, to avoid double sampling.
// Although the overhead it adds is minimal,
// the sample rate passed in shouldn't be set too high
// (e.g. 0.01/1% is probably a good sample rate to use).
//
// It does not count the bytes on the wire directly,
// but reconstructs the request/response with the same thrift protocol.
// As a result, the numbers it reports are not exact numbers,
// but should be good enough to show the overall trend and ballpark numbers.
//
// It also only supports THeaderProtocol.
// If the request is not in THeaderProtocol it does nothing no matter what the
// sample rate is.
//
// For endpoint named "myEndpoint", it reports histograms at:
//
// - payload.size.myEndpoint.request
//
// - payload.size.myEndpoint.response
func ReportPayloadSizeMetrics(rate float64) thrift.ProcessorMiddleware {
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				if rate > 0 {
					// Only report for THeader requests
					if ht, ok := in.Transport().(*thrift.THeaderTransport); ok {
						protoID := ht.Protocol()
						cfg := &thrift.TConfiguration{
							THeaderProtocolID: &protoID,
						}
						var itrans, otrans countingTransport
						transport := thrift.NewTHeaderTransportConf(&itrans, cfg)
						iproto := thrift.NewTHeaderProtocolConf(transport, cfg)
						in = &tDuplicateToProtocol{
							Delegate:    in,
							DuplicateTo: iproto,
						}
						transport = thrift.NewTHeaderTransportConf(&otrans, cfg)
						oproto := thrift.NewTHeaderProtocolConf(transport, cfg)
						out = &tDuplicateToProtocol{
							Delegate:    out,
							DuplicateTo: oproto,
						}

						defer func() {
							iproto.Flush(ctx)
							oproto.Flush(ctx)
							isize := float64(itrans.Size())
							osize := float64(otrans.Size())

							proto := "header-" + tHeaderProtocol2String(protoID)
							labels := prometheus.Labels{
								methodLabel: name,
								protoLabel:  proto,
							}
							payloadSizeRequestBytes.With(labels).Observe(isize)
							payloadSizeResponseBytes.With(labels).Observe(osize)

							if randbp.ShouldSampleWithRate(rate) {
								metricsbp.M.HistogramWithRate(metricsbp.RateArgs{
									Name:             "payload.size." + name + ".request",
									Rate:             1,
									AlreadySampledAt: metricsbp.Float64Ptr(rate),
								}).With("proto", proto).Observe(isize)
								metricsbp.M.HistogramWithRate(metricsbp.RateArgs{
									Name:             "payload.size." + name + ".response",
									Rate:             1,
									AlreadySampledAt: metricsbp.Float64Ptr(rate),
								}).With("proto", proto).Observe(osize)
							}
						}()
					}
				}

				return next.Process(ctx, seqID, in, out)
			},
		}
	}
}

// countingTransport implements thrift.TTransport
type countingTransport struct {
	iobp.CountingSink
}

var _ thrift.TTransport = (*countingTransport)(nil)

func (countingTransport) IsOpen() bool {
	return true
}

// All other functions are unimplemented
func (countingTransport) Close() (err error) { return }

func (countingTransport) Flush(_ context.Context) (err error) { return }

func (countingTransport) Read(_ []byte) (n int, err error) { return }

func (countingTransport) RemainingBytes() (numBytes uint64) { return }

func (countingTransport) Open() (err error) { return }

func tHeaderProtocol2String(proto thrift.THeaderProtocolID) string {
	switch proto {
	default:
		return fmt.Sprintf("%v", proto)
	case thrift.THeaderProtocolCompact:
		return "compact"
	case thrift.THeaderProtocolBinary:
		return "binary"
	}
}

// RecoverPanic recovers from panics raised in the TProccessorFunction chain,
// logs them, and records a metric indicating that the endpoint recovered from a
// panic.
func RecoverPanic(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
	return thrift.WrappedTProcessorFunction{
		Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (ok bool, err thrift.TException) {
			defer func() {
				if r := recover(); r != nil {
					var rErr error
					if asErr, ok := r.(error); ok {
						rErr = asErr
					} else {
						rErr = fmt.Errorf("panic in %q: %+v", name, r)
					}
					log.C(ctx).Errorw(
						"recovered from panic:",
						"err", rErr,
						"endpoint", name,
					)
					metricsbp.M.Counter("panic.recover").With(
						"name", name,
					).Add(1)
					panicRecoverCounter.With(prometheus.Labels{
						methodLabel: name,
					}).Inc()

					// changed named return values to show that the request failed and
					// return the panic value error.
					ok = false
					err = thrift.WrapTException(rErr)
				}
			}()

			return next.Process(ctx, seqId, in, out)
		},
	}
}

// PrometheusServerMiddleware returns middleware to track Prometheus metrics
// specific to the Thrift service.
//
// It emits the following prometheus metrics:
//
// * thrift_server_active_requests gauge with labels:
//
//   - thrift_method: the method of the endpoint called
//
// * thrift_server_latency_seconds histogram with labels above plus:
//
//   - thrift_success: "true" if err == nil, "false" otherwise
//
// * thrift_server_requests_total counter with all labels above plus:
//
//   - thrift_exception_type: the human-readable exception type, e.g.
//     baseplate.Error, etc
//   - thrift_baseplate_status: the numeric status code from a baseplate.Error
//     as a string if present (e.g. 404), or the empty string
//   - thrift_baseplate_status_code: the human-readable status code, e.g.
//     NOT_FOUND, or the empty string
func PrometheusServerMiddleware(method string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
	process := func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (success bool, err thrift.TException) {
		start := time.Now()
		activeRequestLabels := prometheus.Labels{
			methodLabel: method,
		}
		serverActiveRequests.With(activeRequestLabels).Inc()

		defer func() {
			var baseplateStatusCode, baseplateStatus string
			exceptionTypeLabel := stringifyErrorType(err)
			success := prometheusbp.BoolString(err == nil)
			if err != nil {
				var bpErr baseplateErrorCoder
				if errors.As(err, &bpErr) {
					code := bpErr.GetCode()
					baseplateStatusCode = strconv.FormatInt(int64(code), 10)
					if status := baseplate.ErrorCode(code).String(); status != "<UNSET>" {
						baseplateStatus = status
					}
				}
			}

			latencyLabels := prometheus.Labels{
				methodLabel:  method,
				successLabel: success,
			}
			serverLatencyDistribution.With(latencyLabels).Observe(time.Since(start).Seconds())

			totalRequestLabels := prometheus.Labels{
				methodLabel:              method,
				successLabel:             success,
				exceptionLabel:           exceptionTypeLabel,
				baseplateStatusLabel:     baseplateStatus,
				baseplateStatusCodeLabel: baseplateStatusCode,
			}
			serverTotalRequests.With(totalRequestLabels).Inc()
			serverActiveRequests.With(activeRequestLabels).Dec()
		}()

		return next.Process(ctx, seqID, in, out)
	}
	return thrift.WrappedTProcessorFunction{Wrapped: process}
}
