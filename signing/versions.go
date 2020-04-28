package signing

import (
	"encoding/base64"
	"time"

	"github.com/reddit/baseplate.go/secrets"
)

var latest = V1

// Sign calls the latest implementation's Sign function.
func Sign(args SignArgs) (string, error) {
	return latest.Sign(args)
}

type internalVerifyFunc func([]byte, []byte, []secrets.Secret, time.Time) error

// versions is the map from known versions to their implementations.
var versions = map[Version]internalVerifyFunc{
	1: v1Verify,
}

// Verify auto chooses the correct version and verifies the signature with the
// version implementation.
//
// Unrecognized versions will be rejected.
//
// signature should be urlsafe base64 encoded signature, instead of the raw
// one.
//
// If this function returns an error, it will be in the type of VerifyError.
func Verify(message []byte, signature string, secret secrets.VersionedSecret) error {
	buf, err := base64.URLEncoding.DecodeString(signature)
	if err != nil {
		return VerifyError{
			Cause:  err,
			Reason: VerifyErrorReasonBase64,
		}
	}

	v := Version(buf[0])
	verify, ok := versions[v]
	if !ok {
		return VerifyError{
			Reason: VerifyErrorReasonUnknownVersion,
			Data:   v,
		}
	}

	return verify(message, buf, secret.GetAll(), time.Now())
}
