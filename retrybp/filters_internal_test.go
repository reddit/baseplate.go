package retrybp

import (
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
)

var _ thriftRetryableError = (*baseplate.Error)(nil)

type nextFilter struct {
	called bool
}

func (n *nextFilter) filter(_ error) bool {
	n.called = true
	return false
}

func TestRetryableErrorFilter(t *testing.T) {
	e := baseplate.NewError()

	t.Run("unset", func(t *testing.T) {
		var n nextFilter
		e.Retryable = nil
		result := RetryableErrorFilter(e, n.filter)
		if !n.called {
			t.Error("Expected RetryableErrorFilter to call next filter on unset Retryable field, did not happen")
		}
		if result {
			t.Error("Expected false, got true")
		}
	})

	t.Run("true", func(t *testing.T) {
		var n nextFilter
		e.Retryable = thrift.BoolPtr(true)
		result := RetryableErrorFilter(e, n.filter)
		if n.called {
			t.Error("Expected RetryableErrorFilter to make decision without calling next, next called")
		}
		if !result {
			t.Error("Expected true, got false")
		}
	})

	t.Run("false", func(t *testing.T) {
		var n nextFilter
		e.Retryable = thrift.BoolPtr(false)
		result := RetryableErrorFilter(e, n.filter)
		if n.called {
			t.Error("Expected RetryableErrorFilter to make decision without calling next, next called")
		}
		if result {
			t.Error("Expected false, got true")
		}
	})
}
