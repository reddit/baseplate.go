package thriftbp

import (
	"context"
	"errors"

	"github.com/apache/thrift/lib/go/thrift"
	retry "github.com/avast/retry-go"

	"github.com/reddit/baseplate.go/errorsbp"
	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/retrybp"
)

// BaseplateErrorCode defines the minimum interface for a Baseplate Error that is
// used by logic within baseplate.go.
type BaseplateErrorCode interface {
	// GetCode returns the error code describing the general nature of the error.
	GetCode() int32
}

var (
	_ BaseplateErrorCode = (*baseplatethrift.Error)(nil)
)

// ClientPoolConfig errors are returned if the configuration validation fails.
var (
	ErrConfigMissingServiceSlug = errors.New("ServiceSlug cannot be empty")
	ErrConfigMissingAddr        = errors.New("Addr cannot be empty")
	ErrConfigInvalidConnections = errors.New("InitialConnections cannot be bigger than MaxConnections")
)

// WithDefaultRetryableCodes returns a list including the given error codes and
// the default retryable error codes:
//
// 1. TOO_EARLY
//
// 2. SERVICE_UNAVAILABLE
//
// 3. TIMEOUT
func WithDefaultRetryableCodes(codes ...int32) []int32 {
	return append([]int32{
		int32(baseplatethrift.ErrorCode_TOO_EARLY),
		int32(baseplatethrift.ErrorCode_SERVICE_UNAVAILABLE),
		int32(baseplatethrift.ErrorCode_TIMEOUT),
	}, codes...)
}

// BaseplateErrorFilter returns true if the given error is a BaseplateError and
// returns one of the given codes and false if it is a BaseplateError but does
// not return one of the given codes otherwise it calls the next filter in the
// chain.
func BaseplateErrorFilter(codes ...int32) retrybp.Filter {
	codeMap := make(map[int32]bool, len(codes))
	for _, code := range codes {
		codeMap[code] = true
	}
	return func(err error, next retry.RetryIfFunc) bool {
		var bpErr BaseplateErrorCode
		if errors.As(err, &bpErr) {
			return codeMap[bpErr.GetCode()]
		}
		return next(err)
	}
}

// This is an interface implemented by all thrift compiler generated types from
// exceptions defined in .thrift files.
//
// This type might change in the future with thrift compiler changes.
// As a result we don't want to export it to make it part of our API.
type idlException interface {
	thrift.TException

	Read(context.Context, thrift.TProtocol) error
	Write(context.Context, thrift.TProtocol) error

	// This one is important. Without this one thrift.TApplicationException will
	// satisfy this interface as well.
	String() string
}

var _ idlException = (*baseplatethrift.Error)(nil)

// IDLExceptionSuppressor is an errorsbp.Suppressor implementation that returns
// true on errors from exceptions defined in thrift IDL files.
func IDLExceptionSuppressor(err error) bool {
	return errors.As(err, new(idlException))
}

var _ errorsbp.Suppressor = IDLExceptionSuppressor
