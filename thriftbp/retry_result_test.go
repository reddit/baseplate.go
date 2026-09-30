package thriftbp_test

import (
	"context"
	"errors"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/avast/retry-go"
	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/thriftbp"
)

func TestRetryResultLifecycle(t *testing.T) {
	tests := []struct {
		name           string
		firstError     *baseplatethrift.Error
		partialSuccess bool
		secondError    *baseplatethrift.Error
		wantCalls      int
	}{
		{name: "exception then success", firstError: &baseplatethrift.Error{Retryable: thrift.Pointer(true)}, wantCalls: 2},
		{name: "partial response then success", partialSuccess: true, wantCalls: 2},
		{name: "final exception survives", firstError: &baseplatethrift.Error{Retryable: thrift.Pointer(true)}, secondError: &baseplatethrift.Error{Message: thrift.Pointer("final error")}, wantCalls: 2},
		{name: "terminal exception is not cleared", firstError: &baseplatethrift.Error{Retryable: thrift.Pointer(false)}, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result retryResult
			calls := 0
			client := thrift.WrapClient(thrift.WrappedTClient{
				Wrapped: func(_ context.Context, _ string, _, response thrift.TStruct) (thrift.ResponseMeta, error) {
					calls++
					r := response.(*retryResult)
					if r.Error != nil || r.Success != nil {
						t.Fatal("attempt received stale response state")
					}
					if calls == 1 {
						r.Error = tt.firstError
						if tt.partialSuccess {
							r.Success = thrift.Pointer(false)
							return thrift.ResponseMeta{}, thrift.NewTTransportException(thrift.END_OF_FILE, "partial response")
						}
					} else {
						r.Error = tt.secondError
					}
					if r.Error == nil {
						r.Success = thrift.Pointer(true)
					}
					return thrift.ResponseMeta{}, nil
				},
			}, thriftbp.Retry(retry.Attempts(2), retry.Delay(0), retry.RetryIf(func(err error) bool {
				var exception *baseplatethrift.Error
				if errors.As(err, &exception) {
					return exception.GetRetryable()
				}
				return true
			})))
			_, err := client.Call(context.Background(), "test", nil, &result)
			if calls != tt.wantCalls {
				t.Fatalf("calls = %d, want %d", calls, tt.wantCalls)
			}
			wantError := tt.secondError
			if tt.wantCalls == 1 {
				wantError = tt.firstError
			}
			if wantError != nil {
				if !errors.Is(err, wantError) || result.Error != wantError {
					t.Fatalf("final exception = %v, error = %v, want %v", result.Error, err, wantError)
				}
			} else if err != nil || result.Error != nil || result.Success == nil || !*result.Success {
				t.Fatalf("result = %+v, error = %v, want success without exception", result, err)
			}
		})
	}
}

func TestRetryOneway(t *testing.T) {
	calls := 0
	client := thrift.WrapClient(thrift.WrappedTClient{
		Wrapped: func(context.Context, string, thrift.TStruct, thrift.TStruct) (thrift.ResponseMeta, error) {
			calls++
			if calls == 1 {
				return thrift.ResponseMeta{}, thrift.NewTTransportException(thrift.NOT_OPEN, "closed")
			}
			return thrift.ResponseMeta{}, nil
		},
	}, thriftbp.Retry(retry.Attempts(2), retry.Delay(0)))
	if _, err := client.Call(context.Background(), "oneway", nil, nil); err != nil || calls != 2 {
		t.Fatalf("calls = %d, error = %v, want two calls and no error", calls, err)
	}
}

type retryResult struct {
	thrift.TStruct
	Success *bool
	Error   *baseplatethrift.Error
}
