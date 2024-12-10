package faults_test

import (
	"testing"
	"time"

	"github.com/reddit/baseplate.go/internal/faults"
)

const (
	defaultAddress = "testService.testNamespace.svc.cluster.local:12345"
	method         = "testMethod"
	minAbortCode   = 0
	maxAbortCode   = 10
)

type Response struct {
	code    int
	message string
}

func intPtr(i int) *int {
	return &i
}

func TestInjectFault(t *testing.T) {
	testCases := []struct {
		name    string
		address string
		randInt *int

		faultServerAddressHeader   string
		faultServerMethodHeader    string
		faultDelayMsHeader         string
		faultDelayPercentageHeader string
		faultAbortCodeHeader       string
		faultAbortMessageHeader    string
		faultAbortPercentageHeader string

		wantDelayMs  int
		wantResponse *Response
	}{
		{
			name:         "no fault specified",
			wantResponse: nil,
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

			wantResponse: &Response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name:    "invalid server address",
			address: "foo",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultAbortCodeHeader:     "1",
			faultAbortMessageHeader:  "test fault",

			wantResponse: nil,
		},
		{
			name: "server address does not match",

			faultServerAddressHeader: "fooService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultAbortCodeHeader:     "1",
			faultAbortMessageHeader:  "test fault",

			wantResponse: nil,
		},
		{
			name: "method does not match",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "fooMethod",
			faultAbortCodeHeader:     "1",
			faultAbortMessageHeader:  "test fault",

			wantResponse: nil,
		},
		{
			name:    "guaranteed percent",
			randInt: intPtr(99), // Maximum possible integer returned by rand.Intn(100)

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultDelayMsHeader:         "250",
			faultDelayPercentageHeader: "100", // All requests delayed
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "100", // All requests aborted

			wantDelayMs: 250,
			wantResponse: &Response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name:    "fence post below percent",
			randInt: intPtr(49),

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultDelayMsHeader:         "250",
			faultDelayPercentageHeader: "50",
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "50",

			wantDelayMs: 250,
			wantResponse: &Response{
				code:    1,
				message: "test fault",
			},
		},
		{
			name:    "fence post at percent",
			randInt: intPtr(50),

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultDelayMsHeader:         "250",
			faultDelayPercentageHeader: "50",
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "50",

			wantDelayMs:  0,
			wantResponse: nil,
		},
		{
			name:    "guaranteed skip percent",
			randInt: intPtr(0), // Minimum possible integer returned by rand.Intn(100)

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultDelayMsHeader:         "250",
			faultDelayPercentageHeader: "0", // No requests delayed
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "0", // No requests aborted

			wantDelayMs:  0,
			wantResponse: nil,
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
			name: "invalid abort percentage negative",

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "-1",

			wantResponse: nil,
		},
		{
			name: "invalid abort percentage over 100",

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "101",

			wantResponse: nil,
		},
		{
			name: "invalid abort code",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultAbortCodeHeader:     "NaN",
			faultAbortMessageHeader:  "test fault",

			wantResponse: nil,
		},
		{
			name: "less than min abort code",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultAbortCodeHeader:     "-1",
			faultAbortMessageHeader:  "test fault",

			wantResponse: nil,
		},
		{
			name: "greater than max abort code",

			faultServerAddressHeader: "testService.testNamespace",
			faultServerMethodHeader:  "testMethod",
			faultAbortCodeHeader:     "11",
			faultAbortMessageHeader:  "test fault",

			wantResponse: nil,
		},
		{
			name: "invalid abort percentage",

			faultServerAddressHeader:   "testService.testNamespace",
			faultServerMethodHeader:    "testMethod",
			faultAbortCodeHeader:       "1",
			faultAbortMessageHeader:    "test fault",
			faultAbortPercentageHeader: "NaN",

			wantResponse: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			address := tc.address
			if address == "" {
				address = defaultAddress
			}

			getHeaderFn := faults.GetHeaderFn(func(key string) string {
				if key == faults.FaultServerAddressHeader {
					return tc.faultServerAddressHeader
				}
				if key == faults.FaultServerMethodHeader {
					return tc.faultServerMethodHeader
				}
				if key == faults.FaultDelayMsHeader {
					return tc.faultDelayMsHeader
				}
				if key == faults.FaultDelayPercentageHeader {
					return tc.faultDelayPercentageHeader
				}
				if key == faults.FaultAbortCodeHeader {
					return tc.faultAbortCodeHeader
				}
				if key == faults.FaultAbortMessageHeader {
					return tc.faultAbortMessageHeader
				}
				if key == faults.FaultAbortPercentageHeader {
					return tc.faultAbortPercentageHeader
				}
				return ""
			})
			var resumeFn faults.ResumeFn[*Response] = func() (*Response, error) {
				return nil, nil
			}
			var responseFn faults.ResponseFn[*Response] = func(code int, message string) (*Response, error) {
				return &Response{
					code:    code,
					message: message,
				}, nil
			}
			delayMs := 0
			sleepFn := faults.SleepFn(func(d time.Duration) {
				delayMs = int(d.Milliseconds())
			})

			resp, err := faults.InjectFault(faults.InjectFaultParams[*Response]{
				CallerName:   "faults_test.TestInjectFault",
				Address:      address,
				Method:       method,
				AbortCodeMin: minAbortCode,
				AbortCodeMax: maxAbortCode,
				GetHeaderFn:  getHeaderFn,
				ResumeFn:     resumeFn,
				ResponseFn:   responseFn,
				SleepFn:      &sleepFn,
				RandInt:      tc.randInt,
			})

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
