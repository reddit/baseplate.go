package faults

import (
	"context"
	"fmt"
	"strings"
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

func TestParsePercentage(t *testing.T) {
	testCases := []struct {
		name       string
		percentage string
		want       int
		wantErr    string
	}{
		{
			name:       "empty",
			percentage: "",
			want:       100,
		},
		{
			name:       "valid",
			percentage: "50",
			want:       50,
		},
		{
			name:       "NaN",
			percentage: "NaN",
			want:       0,
			wantErr:    "not a valid integer",
		},
		{
			name:       "under min",
			percentage: "-1",
			want:       0,
			wantErr:    "outside the valid range",
		},
		{
			name:       "over max",
			percentage: "101",
			want:       0,
			wantErr:    "outside the valid range",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePercentage(tc.percentage)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
			if tc.wantErr == "" && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error to contain %q, got %v", tc.wantErr, err)
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

	faultServerAddressHeader   string
	faultServerMethodHeader    string
	faultDelayMsHeader         string
	faultDelayPercentageHeader string
	faultAbortCodeHeader       string
	faultAbortMessageHeader    string
	faultAbortPercentageHeader string

	wantDelayMs  int
	wantResponse *response
}

type headers injectTestCase

func (tc *headers) Lookup(_ context.Context, key string) (string, error) {
	if key == FaultServerAddressHeader {
		return tc.faultServerAddressHeader, nil
	}
	if key == FaultServerMethodHeader {
		return tc.faultServerMethodHeader, nil
	}
	if key == FaultDelayMsHeader {
		return tc.faultDelayMsHeader, nil
	}
	if key == FaultDelayPercentageHeader {
		return tc.faultDelayPercentageHeader, nil
	}
	if key == FaultAbortCodeHeader {
		return tc.faultAbortCodeHeader, nil
	}
	if key == FaultAbortMessageHeader {
		return tc.faultAbortMessageHeader, nil
	}
	if key == FaultAbortPercentageHeader {
		return tc.faultAbortPercentageHeader, nil
	}
	return "", fmt.Errorf("header %q not found", key)
}

func TestInject(t *testing.T) {
	testCases := []injectTestCase{
		{
			name: "no fault specified",
		},
		{
			name: "delay",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultDelayMsHeader:       "1",

			wantDelayMs: 1,
		},
		{
			name: "abort",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultAbortCodeHeader:     "1",
			faultAbortMessageHeader:  "test fault",

			wantResponse: &response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name: "server address does not match",

			faultServerAddressHeader: "fooService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultAbortCodeHeader:     "1",
			faultAbortMessageHeader:  "test fault",
		},
		{
			name: "method does not match",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "fooMethod",
			faultAbortCodeHeader:     "1",
			faultAbortMessageHeader:  "test fault",
		},
		{
			name:    "guaranteed percent",
			randInt: 99, // Maximum possible integer returned by rand.Intn(100)

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultDelayMsHeader:         "250",
			faultDelayPercentageHeader: "100", // All requests delayed
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "100", // All requests aborted

			wantDelayMs: 250,
			wantResponse: &response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name:    "fence post below percent",
			randInt: 49,

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultDelayMsHeader:         "250",
			faultDelayPercentageHeader: "50",
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "50",

			wantDelayMs: 250,
			wantResponse: &response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name:    "fence post at percent",
			randInt: 50,

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultDelayMsHeader:         "250",
			faultDelayPercentageHeader: "50",
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "50",

			wantDelayMs: 0,
		},
		{
			name:    "guaranteed skip percent",
			randInt: 0, // Minimum possible integer returned by rand.Intn(100)

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultDelayMsHeader:         "250",
			faultDelayPercentageHeader: "0", // No requests delayed
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "0", // No requests aborted

			wantDelayMs: 0,
		},
		{
			name:    "only skip delay",
			randInt: 50,

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultDelayMsHeader:         "250",
			faultDelayPercentageHeader: "0", // No requests delayed
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "100", // All requests aborted

			wantDelayMs: 0,
			wantResponse: &response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name: "invalid delay percentage negative",

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultDelayMsHeader:         "250",
			faultDelayPercentageHeader: "-1",

			wantDelayMs: 0,
		},
		{
			name: "invalid delay percentage over 100",

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultDelayMsHeader:         "250",
			faultDelayPercentageHeader: "101",

			wantDelayMs: 0,
		},
		{
			name: "invalid delay ms",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultDelayMsHeader:       "NaN",

			wantDelayMs: 0,
		},
		{
			name:     "error while sleeping short circuits",
			sleepErr: true,

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultDelayMsHeader:       "1",
			faultAbortCodeHeader:     "1",
			faultAbortMessageHeader:  "test fault",

			wantDelayMs: 0,
		},
		{
			name: "invalid abort percentage negative",

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "-1",
		},
		{
			name: "invalid abort percentage over 100",

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "101",
		},
		{
			name: "invalid abort code",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultAbortCodeHeader:     "NaN",
			faultAbortMessageHeader:  "test fault",
		},
		{
			name: "less than min abort code",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultAbortCodeHeader:     "-1",
			faultAbortMessageHeader:  "test fault",
		},
		{
			name: "greater than max abort code",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultAbortCodeHeader:     "11",
			faultAbortMessageHeader:  "test fault",
		},
		{
			name: "invalid abort percentage",

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "NaN",
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
			delayMs := 0
			injector.sleep = func(_ context.Context, d time.Duration) error {
				if tc.sleepErr {
					return fmt.Errorf("context cancelled")
				}
				delayMs = int(d.Milliseconds())
				return nil
			}

			resp, err := injector.Inject(context.Background(), address, method, &headers, resume)

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantDelayMs != delayMs {
				t.Fatalf("expected delay of %v ms, got %v ms", tc.wantDelayMs, delayMs)
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
