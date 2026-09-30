package thriftbp_test

import (
	"context"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/avast/retry-go"
	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/thriftbp"
)

func TestRetryReturnsSuccessAfterException(t *testing.T) {
	var result retryResult
	calls := 0
	client := thrift.WrapClient(thrift.WrappedTClient{
		Wrapped: func(_ context.Context, _ string, _, response thrift.TStruct) (thrift.ResponseMeta, error) {
			calls++
			r := response.(*retryResult)
			if calls == 1 {
				r.Error = &baseplatethrift.Error{Retryable: thrift.Pointer(true)}
			} else {
				r.Success = thrift.Pointer(true)
			}
			return thrift.ResponseMeta{}, nil
		},
	}, thriftbp.Retry(retry.Attempts(2), retry.Delay(0)))

	_, err := client.Call(context.Background(), "test", nil, &result)
	if err != nil || calls != 2 {
		t.Fatalf("calls = %d, error = %v, want success after two calls", calls, err)
	}
	if result.Error != nil || result.Success == nil || !*result.Success {
		t.Fatalf("result = %+v, want success without the previous exception", result)
	}
}

type retryResult struct {
	thrift.TStruct
	Success *bool
	Error   *baseplatethrift.Error
}
