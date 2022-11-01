package signing_test

import (
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/signing"
)

func Example() {
	// Should be properly initialized in production code.
	var (
		store      *secrets.Store
		secretPath string

		invalidSignatureCounter prometheus.Counter
		expiredSignatureCounter prometheus.Counter
	)

	secret, _ := store.GetVersionedSecret(secretPath)

	const msg = "Hello, world!"

	// Sign
	signature, _ := signing.Sign(signing.SignArgs{
		Message:   []byte(msg),
		Secret:    secret,
		ExpiresIn: time.Hour,
	})

	// Verify
	err := signing.Verify([]byte(msg), signature, secret)
	if err != nil {
		invalidSignatureCounter.Inc()
		var e signing.VerifyError
		if errors.As(err, &e) {
			switch e.Reason {
			case signing.VerifyErrorReasonExpired:
				expiredSignatureCounter.Inc()
			}
		}
	}
}
