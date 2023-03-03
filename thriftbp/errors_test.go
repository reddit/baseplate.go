package thriftbp_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/avast/retry-go"

	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/thriftbp"
)

func TestWithDefaultRetryableCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		codes    []int32
		expected []int32
	}{
		{
			name: "default",
			expected: []int32{
				int32(baseplatethrift.ErrorCode_TOO_EARLY),
				int32(baseplatethrift.ErrorCode_TOO_MANY_REQUESTS),
				int32(baseplatethrift.ErrorCode_SERVICE_UNAVAILABLE),
			},
		},
		{
			name:  "additional-codes",
			codes: []int32{1, 2, 3},
			expected: []int32{
				int32(baseplatethrift.ErrorCode_TOO_EARLY),
				int32(baseplatethrift.ErrorCode_TOO_MANY_REQUESTS),
				int32(baseplatethrift.ErrorCode_SERVICE_UNAVAILABLE),
				1,
				2,
				3,
			},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				codes := thriftbp.WithDefaultRetryableCodes(c.codes...)

				if !reflect.DeepEqual(codes, c.expected) {
					t.Errorf(
						"code list does not match, expected %+v, got %+v",
						c.expected,
						codes,
					)
				}
			},
		)
	}
}

func fallback(c *counter) retry.RetryIfFunc {
	return func(err error) bool {
		c.incr()
		return false
	}
}

func TestBaseplateErrorFilter(t *testing.T) {
	t.Parallel()

	var (
		retryableCode    int32 = 1
		notRetryableCode int32 = 2
	)

	filter := thriftbp.BaseplateErrorFilter(retryableCode)

	cases := []struct {
		name string
		err  error

		expected       bool
		fallbackCalled bool
	}{
		{
			name: "retryable",
			err: &baseplatethrift.Error{
				Code: &retryableCode,
			},
			expected:       true,
			fallbackCalled: false,
		},
		{
			name: "not-retryable",
			err: &baseplatethrift.Error{
				Code: &notRetryableCode,
			},
			expected:       false,
			fallbackCalled: false,
		},
		{
			name:           "other-error",
			err:            errors.New("test"),
			expected:       false,
			fallbackCalled: true,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				count := counter{}
				result := filter(c.err, fallback(&count))

				if result != c.expected {
					t.Errorf("result mismatch, expected %v, got %v", c.expected, result)
				}
				if c.fallbackCalled && count.count == 0 {
					t.Error("expected fallback to be called but it was not.")
				} else if !c.fallbackCalled && count.count > 0 {
					t.Error("expected fallback to not be called but it was")
				}
			},
		)
	}
}

func TestIDLExceptionSuppressor(t *testing.T) {
	for _, _c := range []struct {
		label    string
		err      error
		expected bool
	}{
		{
			label:    "baseplate.Error",
			err:      baseplatethrift.NewError(),
			expected: true,
		},
		{
			label: "baseplate.Error-500",
			err: &baseplatethrift.Error{
				Code: thrift.Int32Ptr(500),
			},
			expected: false,
		},
		{
			label: "baseplate.Error-400",
			err: &baseplatethrift.Error{
				Code: thrift.Int32Ptr(400),
			},
			expected: true,
		},
		{
			label: "baseplate.Error-1000",
			err: &baseplatethrift.Error{
				Code: thrift.Int32Ptr(1000),
			},
			expected: true,
		},
		{
			label:    "TTransportException",
			err:      thrift.NewTTransportException(0, ""),
			expected: false,
		},
		{
			label:    "TProtocolException",
			err:      thrift.NewTProtocolException(errors.New("")),
			expected: false,
		},
		{
			label:    "TApplicationException",
			err:      thrift.NewTApplicationException(0, ""),
			expected: false,
		},
	} {
		c := _c
		t.Run(c.label, func(t *testing.T) {
			actual := thriftbp.IDLExceptionSuppressor(c.err)
			if actual != c.expected {
				t.Errorf(
					"Expected IDLExceptionSuppressor to return %v for %#v, got %v",
					c.expected,
					c.err,
					actual,
				)
			}
		})
	}
}
