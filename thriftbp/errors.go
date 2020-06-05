package thriftbp

import (
	"errors"

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
