package metricsbp

import (
	"context"
	"testing"

	"github.com/reddit/baseplate.go/tracing"
)

type createServerSpanHook struct {
	hook interface{}
}

func (h createServerSpanHook) OnCreateServerSpan(span *tracing.Span) error {
	span.AddHooks(h.hook)
	return nil
}

func TestCountActiveRequestsHook(t *testing.T) {
	st := NewStatsd(
		context.Background(),
		StatsdConfig{},
	)

	createServerHook := createServerSpanHook{
		hook: countActiveRequestsHook{metrics: st},
	}
	tracing.RegisterCreateServerSpanHooks(createServerHook)
	defer tracing.ResetHooks()

	ctx, span := tracing.StartSpanFromHeaders(context.Background(), "foo", tracing.Headers{})
	counter := st.getActiveRequests()
	if counter != 1 {
		t.Errorf("Expected active requests to be 1 after span started, got %d", counter)
	}
	span.Stop(ctx, nil)
	counter = st.getActiveRequests()
	if counter != 0 {
		t.Errorf("Expected active requests to be 0 after span stopped, got %d", counter)
	}
}
