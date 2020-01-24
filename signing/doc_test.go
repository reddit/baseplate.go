package signing_test

import (
	"errors"
	"time"

	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/signing"
)

func Example() {
	// Should be properly initialized in production code.
	var (
		store      *secrets.Store
		secretPath string
	)

	secret, _ := store.GetVersionedSecret(secretPath)

	const msg = "Hello, world!"

	// Sign
	signature, _ := signing.Sign(signing.SignArgs{
		Message:   []byte(msg),
		Key:       secret.Current,
		ExpiresIn: time.Hour,
	})

	// Verify
	err := signing.Verify([]byte(msg), signature, secret.GetAll()...)
	if err != nil {
		metricsbp.M.Counter("invalid-signature").Add(1)
		var e signing.VerifyError
		if errors.As(err, &e) {
			switch e.Reason {
			case signing.VerifyErrorReasonExpired:
				metricsbp.M.Counter("signature-expired").Add(1)
			}
		}
	}
}
