package httpbp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kit/kit/metrics"

	"github.com/reddit/baseplate.go/metricsbp"
)

type testHandlerPlan struct {
	code int
	err  error
}

func newTestHandler(plan testHandlerPlan) HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		if plan.err != nil {
			return plan.err
		}
		if plan.code != 0 {
			w.WriteHeader(plan.code)
		}
		return nil
	}
}

type statsCounter struct {
	name string
	tags metricsbp.Tags

	v float64
}

func (c *statsCounter) With(labelsAndValues ...string) metrics.Counter {
	if len(labelsAndValues)%2 != 0 {
		panic(fmt.Errorf("uneven labels and values %v", labelsAndValues))
	}
	for i := 0; i < len(labelsAndValues); i += 2 {
		c.tags[labelsAndValues[i]] = labelsAndValues[i+1]
	}
	return c
}

func (c *statsCounter) Add(delta float64) {
	c.v += delta
}

type statsCounterGenerator struct {
	counters []*statsCounter
}

func (gen *statsCounterGenerator) Counter(name string) metrics.Counter {
	c := &statsCounter{
		name: name,
		tags: metricsbp.Tags{},
	}
	gen.counters = append(gen.counters, c)
	return c
}

func TestRecordStatusCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	req, err := http.NewRequest("get", "localhost:9090", strings.NewReader("test"))
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		handler HandlerFunc
		status  string
	}{
		{
			name:    "ok/unset",
			handler: newTestHandler(testHandlerPlan{}),
			status:  "2xx",
		},
		{
			name:    "ok/set",
			handler: newTestHandler(testHandlerPlan{code: http.StatusOK}),
			status:  "2xx",
		},
		{
			name:    "created/set",
			handler: newTestHandler(testHandlerPlan{code: http.StatusCreated}),
			status:  "2xx",
		},
		{
			name:    "moved/set",
			handler: newTestHandler(testHandlerPlan{code: http.StatusMovedPermanently}),
			status:  "3xx",
		},
		{
			name:    "bad-request/set",
			handler: newTestHandler(testHandlerPlan{code: http.StatusBadRequest}),
			status:  "4xx",
		},
		{
			name: "bad-request/un-set/in-error",
			handler: newTestHandler(testHandlerPlan{
				err: JSONError(BadRequest(), nil),
			}),
			status: "4xx",
		},
		{
			name:    "gateway-timeout/set",
			handler: newTestHandler(testHandlerPlan{code: http.StatusGatewayTimeout}),
			status:  "5xx",
		},
		{
			name: "gateway-timeout/un-set/in-error",
			handler: newTestHandler(testHandlerPlan{
				err: JSONError(GatewayTimeout(), nil),
			}),
			status: "5xx",
		},
		{
			name:    "internal-server-err/unset/generic-error",
			handler: newTestHandler(testHandlerPlan{err: fmt.Errorf("oops")}),
			status:  "5xx",
		},
		{
			name: "conflict/code-and-error",
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusNotFound)
				return JSONError(GatewayTimeout(), nil)
			},
			status: "4xx",
		},
		{
			name:    "continue/set",
			handler: newTestHandler(testHandlerPlan{code: http.StatusContinue}),
			status:  "1xx",
		},
		{
			name:    "non-standard/6xx",
			handler: newTestHandler(testHandlerPlan{code: 600}),
			status:  "nan",
		},
		{
			name:    "non-standard/7xx",
			handler: newTestHandler(testHandlerPlan{code: 700}),
			status:  "nan",
		},
		{
			name:    "non-standard/8xx",
			handler: newTestHandler(testHandlerPlan{code: 800}),
			status:  "nan",
		},
		{
			name:    "non-standard/9xx",
			handler: newTestHandler(testHandlerPlan{code: 999}),
			status:  "nan",
		},

		// Note, while these tests show that the "nan" responses for
		// invalid status codes work, the default ResponseWriter you get from Go
		// will panic if you give it an invalid status code so using errors like
		// this would result in a panic later in your service. This is also why we
		// have to use an HTTPError to test this, if we tried just setting the code
		// using w.WriteHeader, it would panic.
		{
			name: "invalid/nan/small",
			handler: newTestHandler(testHandlerPlan{
				err: JSONError(NewErrorResponse(10, "", ""), nil),
			}),
			status: "nan",
		},
		{
			name: "invalid/nan/negative",
			handler: newTestHandler(testHandlerPlan{
				err: JSONError(NewErrorResponse(-200, "", ""), nil),
			}),
			status: "nan",
		},
		{
			name: "invalid/nan/large",
			handler: newTestHandler(testHandlerPlan{
				err: JSONError(NewErrorResponse(1000, "", ""), nil),
			}),
			status: "nan",
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(c.name, func(t *testing.T) {
			gen := &statsCounterGenerator{}
			handle := Wrap("test", c.handler, recordStatusCode(gen))
			handle(ctx, httptest.NewRecorder(), req)
			if len(gen.counters) != 1 {
				t.Fatalf("expected to have 1 counter, got %v", gen.counters)
			}
			count := gen.counters[0]
			if name := "baseplate.http.test.response"; name != count.name {
				t.Errorf("name mismatch, expected %q, got %q", name, count.name)
			}
			if count.tags["status"] != c.status {
				t.Errorf("status tag mismatch, expected %q, got %q", c.status, count.tags["status"])
			}
			if count.v != 1 {
				t.Errorf("value mismatch, expected 1.0, got %f", count.v)
			}
		})
	}
}
