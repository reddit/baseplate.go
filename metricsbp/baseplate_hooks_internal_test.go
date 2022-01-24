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
		Config{
			BufferInMemoryForTesting: true,
		},
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

func TestSplitClientSpanName(t *testing.T) {
	for _, c := range []struct {
		span     string
		client   string
		endpoint string
	}{
		{
			span:     "foo.bar",
			client:   "foo",
			endpoint: "bar",
		},
		{
			span:     "foo-with-retry.bar",
			client:   "foo-with-retry",
			endpoint: "bar",
		},
		{
			span:     "",
			client:   "",
			endpoint: "",
		},
		{
			span:     "foo",
			client:   "",
			endpoint: "foo",
		},
		{
			span:     "foo..bar",
			client:   "foo.",
			endpoint: "bar",
		},
		{
			span:     "foo..",
			client:   "foo.",
			endpoint: "",
		},
		{
			span:     ".foo",
			client:   "",
			endpoint: "foo",
		},
		{
			span:     "..foo",
			client:   ".",
			endpoint: "foo",
		},
		{
			span:     "1.2.3",
			client:   "1.2",
			endpoint: "3",
		},
	} {
		t.Run(c.span, func(t *testing.T) {
			client, endpoint := splitClientSpanName(c.span)
			if client != c.client {
				t.Errorf("Expected client name of %q to be %q, got %q", c.span, c.client, client)
			}
			if endpoint != c.endpoint {
				t.Errorf("Expected endpoint name of %q to be %q, got %q", c.span, c.endpoint, endpoint)
			}
		})
	}
}
