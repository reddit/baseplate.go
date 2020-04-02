package thriftbp_test

import (
	"context"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/reddit/baseplate.go/thriftbp"
)

type counter struct {
	count int
}

func (c *counter) Incr() {
	c.count++
}

func testMiddleware(c *counter) thriftbp.Middleware {
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thriftbp.WrappedTProcessorFunc{
			Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				c.Incr()
				return next.Process(ctx, seqId, in, out)
			},
		}
	}
}

func TestWrap(t *testing.T) {
	name := "test"
	processor := thriftbp.NewMockBaseplateProcessor(
		map[string]thrift.TProcessorFunction{
			name: thriftbp.WrappedTProcessorFunc{
				Wrapped: func(ctx context.Context, seqId int32, in, out thrift.TProtocol) (bool, thrift.TException) {
					return true, nil
				},
			},
		},
	)
	c := &counter{}
	if c.count != 0 {
		t.Fatal("Unexpected initial count.")
	}
	ctx := context.Background()
	ctx = thrift.SetHeader(ctx, thriftbp.HeaderTracingSampled, thriftbp.HeaderTracingSampledTrue)
	ctx = thriftbp.SetMockBaseplateProcessorName(ctx, name)
	wrapped := thriftbp.Wrap(processor, testMiddleware(c))
	wrapped.Process(ctx, nil, nil)
	if c.count != 1 {
		t.Fatalf("Unexpected count value %v", c.count)
	}
}
