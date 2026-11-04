package tracing

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"

	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
	"github.com/reddit/baseplate.go/log"
)

func TestServerSpanCompatibility(t *testing.T) {
	for _, useV2 := range []bool{false, true} {
		t.Run(strconv.FormatBool(useV2), func(t *testing.T) {
			previousTracer, previousGlobal := globalTracer, opentracing.GlobalTracer()
			previousHooks := createServerSpanHooks
			recorder := new(countingRecorder)
			globalTracer = Tracer{sampleRate: 1, logger: log.NopWrapper, recorder: recorder}
			opentracing.SetGlobalTracer(&globalTracer)
			hook := new(compatibilityHook)
			createServerSpanHooks = []CreateServerSpanHook{hook}
			previousProvider := internalv2compat.SetV2TracingServerSpanProvider(nil)
			t.Cleanup(func() {
				internalv2compat.SetV2TracingServerSpanProvider(previousProvider)
				globalTracer = previousTracer
				opentracing.SetGlobalTracer(previousGlobal)
				createServerSpanHooks = previousHooks
			})
			foreign := mocktracer.New()
			if useV2 {
				internalv2compat.SetV2TracingServerSpanProvider(func(ctx context.Context, name string) (context.Context, internalv2compat.ServerSpan) {
					s, ctx := opentracing.StartSpanFromContextWithTracer(ctx, foreign, name)
					mock := s.(*mocktracer.MockSpan)
					return ctx, internalv2compat.ServerSpan{
						Span:      s,
						TraceID:   strconv.Itoa(mock.SpanContext.TraceID),
						SpanID:    strconv.Itoa(mock.SpanContext.SpanID),
						Sampled:   true,
						StartTime: mock.StartTime,
					}
				})
			}
			ctx, span := StartTopLevelServerSpan(context.Background(), "consumer.example")
			span.SetTag("topic", "example")
			span.AddCounter("messages", 2)
			span.AddCounter("messages", 3)
			finish := time.Now()
			opts := FinishOptions{Ctx: ctx, Err: context.DeadlineExceeded}.Convert()
			opts.FinishTime = finish
			span.FinishWithOptions(opts)
			if diff := cmp.Diff([]string{"create", "start", "tag", "counter", "counter", "stop"}, hook.calls); diff != "" {
				t.Errorf("hooks (-want +got):\n%s", diff)
			}
			if hook.err != context.DeadlineExceeded || span.Name() != "consumer.example" ||
				span.SpanType() != SpanTypeServer || !span.StopTime().Equal(finish) {

				t.Error("legacy metadata or error hooks were lost")
			}
			if useV2 {
				finished := foreign.FinishedSpans()
				if len(finished) != 1 {
					t.Fatalf("finished %d spans, want 1", len(finished))
				}
				actual := finished[0]
				if actual != opentracing.SpanFromContext(ctx) ||
					span.ID() != strconv.Itoa(actual.SpanContext.SpanID) ||
					span.TraceID() != strconv.Itoa(actual.SpanContext.TraceID) ||
					!span.StartTime().Equal(actual.StartTime) || !finish.Equal(actual.FinishTime) {

					t.Error("returned wrapper does not identify and finish the context span")
				}
				want := map[string]interface{}{
					"topic":                          "example",
					"counter.messages":               float64(5),
					ZipkinBinaryAnnotationKeyError:   true,
					ZipkinBinaryAnnotationKeyTimeOut: true,
				}
				if diff := cmp.Diff(want, actual.Tags()); diff != "" {
					t.Errorf("annotations (-want +got):\n%s", diff)
				}
				if recorder.calls != 0 {
					t.Errorf("published %d duplicate legacy spans", recorder.calls)
				}
			} else if recorder.calls != 1 || opentracing.SpanFromContext(ctx) != span {
				t.Error("legacy tracing changed without a v2 provider")
			}
		})
	}
}

type countingRecorder struct{ calls int }

func (r *countingRecorder) Close() error                       { return nil }
func (r *countingRecorder) Send(context.Context, []byte) error { r.calls++; return nil }

type compatibilityHook struct {
	calls []string
	err   error
}

func (h *compatibilityHook) OnCreateServerSpan(s *Span) error {
	h.calls = append(h.calls, "create")
	s.AddHooks(h)
	return nil
}
func (h *compatibilityHook) OnPostStart(*Span) error { h.calls = append(h.calls, "start"); return nil }
func (h *compatibilityHook) OnPreStop(_ *Span, err error) error {
	h.calls = append(h.calls, "stop")
	h.err = err
	return nil
}
func (h *compatibilityHook) OnSetTag(*Span, string, interface{}) error {
	h.calls = append(h.calls, "tag")
	return nil
}
func (h *compatibilityHook) OnAddCounter(*Span, string, float64) error {
	h.calls = append(h.calls, "counter")
	return nil
}
