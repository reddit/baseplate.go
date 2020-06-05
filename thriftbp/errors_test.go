package thriftbp_test

import (
	"errors"
	"reflect"
	"testing"

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
				int32(baseplatethrift.ErrorCode_SERVICE_UNAVAILABLE),
				int32(baseplatethrift.ErrorCode_TIMEOUT),
			},
		},
		{
			name:  "additional-codes",
			codes: []int32{1, 2, 3},
			expected: []int32{
				int32(baseplatethrift.ErrorCode_TOO_EARLY),
				int32(baseplatethrift.ErrorCode_SERVICE_UNAVAILABLE),
				int32(baseplatethrift.ErrorCode_TIMEOUT),
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
