package thriftbp

import (
	"errors"

	"github.com/apache/thrift/lib/go/thrift"
	retry "github.com/avast/retry-go"

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

// NewBaseplateError is a helper function for creating baseplate.Error thrift
// objects to avoid manual type-conversion that the generated Thrift code requires.
//
// detailsKeysValues is used to populate baseplate.Error.Details and should have
// an even number of values. If it has an odd number of values the last value
// will be discarded. The first entry in each pair of values will be the key and
// the second entry will be the value. If no values are provided, the
// baseplate.Error will be initialized with an empty map as Details rather than
// nil.
func NewBaseplateError(code baseplatethrift.ErrorCode, message string, detailsKeysValues ...string) *baseplatethrift.Error {
	details := make(map[string]string, len(detailsKeysValues)/2)
	for i := 0; i+1 < len(detailsKeysValues); i += 2 {
		key, value := detailsKeysValues[i], detailsKeysValues[i+1]
		details[key] = value
	}
	return &baseplatethrift.Error{
		Code:    thrift.Int32Ptr(int32(code)),
		Message: &message,
		Details: details,
	}
}
