package thriftint_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/internal/thriftint"
)

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
		{
			label: "already-wrapped",
			orig: fmt.Errorf("already wrapped: %w", thriftint.WrapBaseplateError(&baseplatethrift.Error{
				Message:   thrift.StringPtr("message"),
				Code:      thrift.Int32Ptr(1),
				Retryable: thrift.BoolPtr(true),
			})),
			expected: `already wrapped: baseplate.Error: "message" (code=1, retryable=true)`,
		},
	} {
		c := _c
		t.Run(c.label, func(t *testing.T) {
			err := thriftint.WrapBaseplateError(c.orig)
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

		err := thriftint.WrapBaseplateError(&baseplatethrift.Error{
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
