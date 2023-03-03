// Package thriftint provides internal thrift related helpers to avoid circular
// dependencies.
package thriftint

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
)

// baseplateError defines the interface of thrift compiled baseplate.Error
// that will be satisfied as long as services are using the same version of
// thrift compiler.
type baseplateError interface {
	thrift.TException

	IsSetMessage() bool
	GetMessage() string

	IsSetCode() bool
	GetCode() int32

	IsSetRetryable() bool
	GetRetryable() bool

	IsSetDetails() bool
	GetDetails() map[string]string
}

var (
	_ baseplateError = (*baseplate.Error)(nil)
)

type wrappedBaseplateError struct {
	cause error
	bpErr baseplateError
}

func (e wrappedBaseplateError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("baseplate.Error: %q", e.bpErr.GetMessage()))
	var details []string
	if e.bpErr.IsSetCode() {
		details = append(details, fmt.Sprintf("code=%d", e.bpErr.GetCode()))
	}
	if e.bpErr.IsSetRetryable() {
		details = append(details, fmt.Sprintf("retryable=%v", e.bpErr.GetRetryable()))
	}
	if e.bpErr.IsSetDetails() {
		details = append(details, fmt.Sprintf("details=%#v", e.bpErr.GetDetails()))
	}
	if len(details) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(details, ", "))
		sb.WriteString(")")
	}
	return sb.String()
}

func (e wrappedBaseplateError) Unwrap() error {
	return e.cause
}

// WrapBaseplateError wraps e to an error with more meaningful error message if
// e is baseplate.Error. Otherwise it returns e as-is.
//
// NOTE: This in general should only be used in clients.
// If you wrap baseplate.Error returned in server code,
// it could cause the error no longer being recognized as an error defined in
// thrift IDL by the thrift server, and the client would get a generic
// TApplicationException instead.
// If you only need this for logging, it's auto applied on zapcore initialized
// from log package automatically, and you don't need to do anything special
// about it.
func WrapBaseplateError(e error) error {
	if errors.As(e, new(wrappedBaseplateError)) {
		// Already wrapped, return e as-is to avoid double wrapping
		return e
	}

	var bpErr baseplateError
	if errors.As(e, &bpErr) {
		return wrappedBaseplateError{
			cause: e,
			bpErr: bpErr,
		}
	}
	return e
}
