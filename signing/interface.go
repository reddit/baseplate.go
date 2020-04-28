package signing

import (
	"fmt"
	"strings"
	"time"

	"github.com/reddit/baseplate.go/secrets"
)

// The Version of the signing protocol.
type Version uint8

// SignArgs are the args passed into Version.Sign function.
type SignArgs struct {
	// The message to sign. Required.
	Message []byte

	// The secret used to sign the message. Required.
	Secret secrets.VersionedSecret

	// Signature expiring time.
	//
	// V1: If ExpiresAt is non-zero, it will be used.
	// Otherwise time.Now().Add(ExpiresIn) will be used.
	// Note that V1 only defined second precision,
	// any sub-second precision in ExpiresIn or ExpiresAt will be dropped and
	// rounded down.
	ExpiresAt time.Time
	ExpiresIn time.Duration
}

// The Interface of a concrete version of Baseplate message signing spec.
type Interface interface {
	// Sign a message. Returns urlsafe base64 encoded signature.
	Sign(args SignArgs) (string, error)

	// Verify a signature.
	//
	// signature should be urlsafe base64 encoded signature, instead of the raw
	// one.
	//
	// If this function returns an error, it will be in the type of VerifyError.
	Verify(message []byte, signature string, secret secrets.VersionedSecret) error
}

// VerifyError is the error type returned by Version.Verify.
type VerifyError struct {
	// The underlying error, could be nil.
	Cause error

	// The reason of the error.
	Reason VerifyErrorReason

	// The additional data to the error, could be empty.
	Data interface{}
}

// VerifyErrorReason is an enum type for common reasons in VerifyError.
type VerifyErrorReason int

// VerifyErrorReason values
const (
	// A catch-all reason not belong to any of the other reasons below.
	VerifyErrorReasonOther VerifyErrorReason = iota

	// The signature is in an unknown version.
	VerifyErrorReasonUnknownVersion

	// Base64 decoding of the signature failed.
	VerifyErrorReasonBase64

	// The signature has expired.
	// Note that this does not necessarily mean the signature matches.
	VerifyErrorReasonExpired

	// The signature doesn't match.
	VerifyErrorReasonMismatch
)

// Unwrap returns the underlying error, if any.
func (e VerifyError) Unwrap() error {
	return e.Cause
}

func (e VerifyError) Error() string {
	var sb strings.Builder
	sb.WriteString("signing: ")
	switch e.Reason {
	default:
		if msg, ok := e.Data.(string); ok {
			sb.WriteString(msg)
		} else {
			sb.WriteString("verification failed")
		}
	case VerifyErrorReasonUnknownVersion:
		sb.WriteString("unrecognized version")
		if ver, ok := e.Data.(Version); ok {
			sb.WriteString(fmt.Sprintf(": %d", ver))
		}
	case VerifyErrorReasonBase64:
		sb.WriteString("base64 decoding failed")
	case VerifyErrorReasonExpired:
		sb.WriteString("signature expired")
	case VerifyErrorReasonMismatch:
		sb.WriteString("signature mismatch")
	}
	if e.Cause != nil {
		sb.WriteString(": ")
		sb.WriteString(e.Cause.Error())
	}
	return sb.String()
}
