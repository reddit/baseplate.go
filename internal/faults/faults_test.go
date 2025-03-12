package faults

import (
	"context"
	"fmt"
	"testing"
	"time"
)

const (
	address      = "testService.testNamespace.svc.cluster.local:12345"
	method       = "testMethod"
	minAbortCode = 0
	maxAbortCode = 10
)

func TestGetCanonicalAddress(t *testing.T) {
	testCases := []struct {
		name    string
		address string
		want    string
	}{
		{
			name:    "cluster local address",
			address: "testService.testNamespace.svc.cluster.local:12345",
			want:    "testService.testNamespace",
		},
		{
			name:    "external address port stripped",
			address: "foo.bar:12345",
			want:    "foo.bar",
		},
		{
			name:    "unexpected address path stripped",
			address: "foo.bar:12345/path",
			want:    "foo.bar",
		},
		{
			name:    "unexpected trailing colon untouched",
			address: "foo.bar:",
			want:    "foo.bar:",
		},
		{
			name:    "external address without port untouched",
			address: "unix://foo",
			want:    "unix://foo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := getCanonicalAddress(tc.address)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

type response struct {
	code    int
	message string
}
type injectTestCase struct {
	name     string
	randInt  int
	sleepErr bool

	faultHeader string

	wantDelay    time.Duration
	wantResponse *response
}

type headers injectTestCase

func (tc *headers) LookupValues(_ context.Context, key string) ([]string, error) {
	if key != FaultHeader {
		return []string{}, fmt.Errorf("header %q not found", key)
	}
	return []string{tc.faultHeader}, nil
}

func TestInject(t *testing.T) {
	testCases := []injectTestCase{
		{
			name: "no fault specified",
		},
		{
			name:        "delay",
			faultHeader: "a=testService.testNamespace;m=testMethod;d=1",

			wantDelay: 1 * time.Millisecond,
		},
		{
			name:        "abort",
			faultHeader: "a=testService.testNamespace;m=testMethod;f=1;b=test fault",

			wantResponse: &response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name:        "abort with multiple header values",
			faultHeader: "a=fooService.testNamespace;f=2, a=testService.testNamespace;m=testMethod;f=1;b=test fault, a=barService.testNamespace;f=2",

			wantResponse: &response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name:        "server address does not match",
			faultHeader: "a=fooService.testNamespace;m=testMethod;f=1;b=test fault",
		},
		{
			name:        "method does not match",
			faultHeader: "a=testService.testNamespace;m=fooMethod;f=1;b=test fault",
		},
		{
			name:    "guaranteed percent",
			randInt: 99, // Maximum possible integer returned by rand.Intn(100)

			// All requests delayed and aborted.
			faultHeader: "a=testService.testNamespace;m=testMethod;d=250;D=100;f=1;b=test fault;F=100",

			wantDelay: 250 * time.Millisecond,
			wantResponse: &response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name:        "fence post below percent",
			randInt:     49,
			faultHeader: "a=testService.testNamespace;m=testMethod;d=250;D=50;f=1;b=test fault;F=50",

			wantDelay: 250 * time.Millisecond,
			wantResponse: &response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name:        "fence post at percent",
			randInt:     50,
			faultHeader: "a=testService.testNamespace;m=testMethod;d=250;D=50;f=1;b=test fault;F=50",

			wantDelay: 0,
		},
		{
			name:    "guaranteed skip percent",
			randInt: 0, // Minimum possible integer returned by rand.Intn(100)

			// No requests delayed or aborted.
			faultHeader: "a=testService.testNamespace;m=testMethod;d=250;D=0;f=1;b=test fault;F=0",

			wantDelay: 0,
		},
		{
			name:    "only skip delay",
			randInt: 50,

			// No requests delayed; all requests aborted.
			faultHeader: "a=testService.testNamespace;m=testMethod;d=250;D=0;f=1;b=test fault;F=100",

			wantDelay: 0,
			wantResponse: &response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name:        "invalid header value",
			faultHeader: "foo",

			wantDelay: 0,
		},
		{
			name:        "error while sleeping short circuits",
			sleepErr:    true,
			faultHeader: "a=testService.testNamespace;m=testMethod;d=1;f=1;b=test fault",

			wantDelay: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			injector := NewInjector(
				"TestClient",
				"faults_test.TestInjectFault",
				minAbortCode,
				maxAbortCode,
				WithDefaultAbort(func(code int, message string) (*response, error) {
					return &response{
						code:    code,
						message: message,
					}, nil
				}),
			)

			headers := headers(tc)

			var resume Resume[*response] = func() (*response, error) {
				return nil, nil
			}

			// Override the selected and sleep functions for testing.
			injector.selected = func(percentage int) bool {
				return tc.randInt < percentage
			}
			delay := time.Duration(0)
			injector.sleep = func(_ context.Context, d time.Duration) error {
				if tc.sleepErr {
					return fmt.Errorf("context cancelled")
				}
				delay = d
				return nil
			}

			resp, err := injector.Inject(context.Background(), address, method, &headers, resume)

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantDelay != delay {
				t.Fatalf("expected delay of %v ms, got %v ms", tc.wantDelay, delay)
			}
			if tc.wantResponse == nil && resp != nil {
				t.Fatalf("expected no response, got %v", resp)
			}
			if tc.wantResponse != nil && resp == nil {
				t.Fatalf("expected response %v, got nil", tc.wantResponse)
			}
			if resp != nil && *tc.wantResponse != *resp {
				t.Fatalf("expected response %v, got %v", tc.wantResponse, resp)
			}
		})
	}
}
