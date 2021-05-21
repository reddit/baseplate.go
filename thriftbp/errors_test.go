package thriftbp_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	retry "github.com/avast/retry-go"

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

func TestWrapBaseplateError(t *testing.T) {
	for _, _c := range []struct {
		label    string
		orig     error
		expected string
	}{
		{
			label:    "nil",
			orig:     nil,
			expected: "<nil>",
		},
		{
			label:    "not-bp",
			orig:     errors.New("foo"),
			expected: "foo",
		},
		{
			label:    "empty",
			orig:     &baseplatethrift.Error{},
			expected: `baseplate.Error: ""`,
		},
		{
			label: "full",
			orig: &baseplatethrift.Error{
				Message:   thrift.StringPtr("message"),
				Code:      thrift.Int32Ptr(1),
				Retryable: thrift.BoolPtr(true),
				Details: map[string]string{
					"foo": "bar",
				},
			},
			expected: `baseplate.Error: "message" (code=1, retryable=true, details=map[string]string{"foo":"bar"})`,
		},
		{
			label: "message-only",
			orig: &baseplatethrift.Error{
				Message: thrift.StringPtr("message"),
			},
			expected: `baseplate.Error: "message"`,
		},
		{
			label: "no-message",
			orig: &baseplatethrift.Error{
				Code:      thrift.Int32Ptr(1),
				Retryable: thrift.BoolPtr(true),
				Details: map[string]string{
					"foo": "bar",
				},
			},
			expected: `baseplate.Error: "" (code=1, retryable=true, details=map[string]string{"foo":"bar"})`,
		},
		{
			label: "no-code",
			orig: &baseplatethrift.Error{
				Message:   thrift.StringPtr("message"),
				Retryable: thrift.BoolPtr(true),
				Details: map[string]string{
					"foo": "bar",
				},
			},
			expected: `baseplate.Error: "message" (retryable=true, details=map[string]string{"foo":"bar"})`,
		},
		{
			label: "no-retryable",
			orig: &baseplatethrift.Error{
				Message: thrift.StringPtr("message"),
				Code:    thrift.Int32Ptr(1),
				Details: map[string]string{
					"foo": "bar",
				},
			},
			expected: `baseplate.Error: "message" (code=1, details=map[string]string{"foo":"bar"})`,
		},
		{
			label: "no-details",
			orig: &baseplatethrift.Error{
				Message:   thrift.StringPtr("message"),
				Code:      thrift.Int32Ptr(1),
				Retryable: thrift.BoolPtr(true),
			},
			expected: `baseplate.Error: "message" (code=1, retryable=true)`,
		},
	} {
		c := _c
		t.Run(c.label, func(t *testing.T) {
			err := thriftbp.WrapBaseplateError(c.orig)
			actual := fmt.Sprintf("%v", err)
			if c.expected != actual {
				t.Errorf("Error message expected %q, got %q", c.expected, actual)
			}
		})
	}

	t.Run("errorsAs", func(t *testing.T) {
		// Copied from retrybp package
		type thriftRetryableError interface {
			error

			IsSetRetryable() bool
			GetRetryable() bool
		}

		err := thriftbp.WrapBaseplateError(&baseplatethrift.Error{
			Message:   thrift.StringPtr("message"),
			Code:      thrift.Int32Ptr(1),
			Retryable: thrift.BoolPtr(true),
			Details: map[string]string{
				"foo": "bar",
			},
		})
		if !errors.As(err, new(*baseplatethrift.Error)) {
			t.Errorf("%v cannot be casted into *baseplate.Error", err)
		}
		if !errors.As(err, new(thriftRetryableError)) {
			t.Errorf("%v cannot be casted into thriftRetryableError", err)
		}
	})
}
