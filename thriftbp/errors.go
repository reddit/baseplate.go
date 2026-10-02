package thriftbp

import (
	"errors"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/avast/retry-go"

	"github.com/reddit/baseplate.go/errorsbp"
	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/internal/thriftint"
	"github.com/reddit/baseplate.go/retrybp"
)

// We create this interface instead of using baseplateError because we want it to always match
// regardless of the version of the thrift file the user has put in their repository.
type baseplateErrorCoder interface {
	thrift.TException

	IsSetCode() bool
	GetCode() int32
}

var (
	_ baseplateErrorCoder = (*baseplatethrift.Error)(nil)
)

// ClientPoolConfig errors are returned if the configuration validation fails.
var (
	ErrConfigMissingServiceSlug    = errors.New("`ServiceSlug` cannot be empty")
	ErrConfigMissingAddr           = errors.New("`Addr` cannot be empty")
	ErrConfigInvalidConnections    = errors.New("`InitialConnections` cannot be bigger than `MaxConnections`")
	ErrConfigInvalidMinConnections = errors.New("`MinConnections` cannot be bigger than `MaxConnections`")
)

// WithDefaultRetryableCodes returns a list including the given error codes and
// the default retryable error codes:
//
// 1. TOO_EARLY
//
// 2. TOO_MANY_REQUESTS
//
// 3. SERVICE_UNAVAILABLE
func WithDefaultRetryableCodes(codes ...int32) []int32 {
	return append([]int32{
		int32(baseplatethrift.ErrorCode_TOO_EARLY),
		int32(baseplatethrift.ErrorCode_TOO_MANY_REQUESTS),
		int32(baseplatethrift.ErrorCode_SERVICE_UNAVAILABLE),
	}, codes...)
}

// BaseplateErrorFilter returns true if the given error is a baseplate.Error
// and returns one of the given codes and false if it is a baseplate.Error
// but does not return one of the given codes otherwise it calls the next filter
// in the chain.
func BaseplateErrorFilter(codes ...int32) retrybp.Filter {
	codeMap := make(map[int32]bool, len(codes))
	for _, code := range codes {
		codeMap[code] = true
	}
	return func(err error, next retry.RetryIfFunc) bool {
		var bpErr baseplateErrorCoder
		if errors.As(err, &bpErr) {
			return codeMap[bpErr.GetCode()]
		}
		return next(err)
	}
}

// IDLExceptionSuppressor is an errorsbp.Suppressor implementation that returns
// true on errors from exceptions defined in thrift IDL files.
//
// Note that if the exception is baseplate.Error,
// this function will NOT suppress it if the code is in range [500, 600).
func IDLExceptionSuppressor(err error) bool {
	var te thrift.TException
	if !errors.As(err, &te) {
		return false
	}
	var bpErr baseplateErrorCoder
	if errors.As(err, &bpErr) {
		// If this is also baseplate.Error,
		// only suppress it if the error code is outside of [500, 600).
		code := bpErr.GetCode()
		return code < 500 || code >= 600
	}
	return te.TExceptionType() == thrift.TExceptionTypeCompiled
}

var _ errorsbp.Suppressor = IDLExceptionSuppressor

// WrapBaseplateError wraps *baseplate.Error into errors with better error
// message, and can be unwrapped to the original *baseplate.Error.
//
// If e is not *baseplate.Error it will be returned as-is.
//
// For logging this wrapping is auto applied as long as you initialize zap
// logger from log package so this is not needed.
func WrapBaseplateError(e error) error {
	return thriftint.WrapBaseplateError(e)
}
