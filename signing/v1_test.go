package signing

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/secrets"
)

type verifyFunc func(message []byte, signature string, secret secrets.VersionedSecret) error

func TestV1(t *testing.T) {
	var e VerifyError

	msg := []byte("Hello, world!")
	secret := secrets.VersionedSecret{Current: secrets.Secret("hunter2")}
	invalidSecret := secrets.VersionedSecret{Current: secrets.Secret("hunter0")}
	expiration := time.Now().Add(time.Hour * 24)

	var validSig string

	t.Run(
		"sign",
		func(t *testing.T) {
			sig, err := V1.Sign(SignArgs{
				Message:   msg,
				Secret:    secret,
				ExpiresAt: expiration,
			})
			if err != nil {
				t.Fatal(err)
			}
			validSig = sig
		},
	)

	if validSig == "" {
		t.Fatal("signing failed")
	}

	t.Run(
		"expired",
		func(t *testing.T) {
			rawSig, err := base64.URLEncoding.DecodeString(validSig)
			if err != nil {
				t.Fatal(err)
			}
			err = v1Verify(msg, rawSig, secret.GetAll(), expiration.Add(time.Second))
			if !errors.As(err, &e) {
				t.Errorf("Expected VerifyError, got %v", err)
			}
			if e.Reason != VerifyErrorReasonExpired {
				t.Errorf("Expected VerifyError with reason expired, got %v", e)
			}
		},
	)

	verifyFuncs := map[string]verifyFunc{
		"V1.Verify": V1.Verify,
		"Verify":    Verify,
	}

	for label, verify := range verifyFuncs {
		t.Run(
			label,
			func(t *testing.T) {
				t.Run(
					"length-mismatch",
					func(t *testing.T) {
						t.Run(
							"short",
							func(t *testing.T) {
								// This signature should still be base64 decodable.
								sig := validSig[:V1SignatureLength-4]
								err := verify(msg, sig, secret)
								if !errors.As(err, &e) {
									t.Errorf("Expected VerifyError, got %v", err)
								}
							},
						)

						t.Run(
							"long",
							func(t *testing.T) {
								// This signature should still be base64 decodable.
								sig := validSig + "===="
								err := verify(msg, sig, secret)
								if !errors.As(err, &e) {
									t.Errorf("Expected VerifyError, got %v", err)
								}
							},
						)
					},
				)

				t.Run(
					"base64-invalid",
					func(t *testing.T) {
						// Replace the last character of validSig to "/"
						sig := validSig[:V1SignatureLength-1] + "/"
						err := verify(msg, sig, secret)
						if !errors.As(err, &e) {
							t.Errorf("Expected VerifyError, got %v", err)
						}
						if e.Reason != VerifyErrorReasonBase64 {
							t.Errorf("Expected VerifyError with reason base64, got %v", e)
						}
					},
				)

				t.Run(
					"mismatch",
					func(t *testing.T) {
						err := verify(msg, validSig, invalidSecret)
						if !errors.As(err, &e) {
							t.Errorf("Expected VerifyError, got %v", err)
						}
						if e.Reason != VerifyErrorReasonMismatch {
							t.Errorf("Expected VerifyError with reason mismatch, got %v", e)
						}
					},
				)

				t.Run(
					"key-rotation",
					func(t *testing.T) {
						rotating := secrets.VersionedSecret{
							Current:  invalidSecret.Current,
							Previous: secret.Current,
						}
						err := verify(msg, validSig, rotating)
						if err != nil {
							t.Errorf("Expected nil error, got %v", err)
						}
					},
				)

				t.Run(
					"unrecognized-version",
					func(t *testing.T) {
						rawSig, err := base64.URLEncoding.DecodeString(validSig)
						if err != nil {
							t.Fatal(err)
						}
						// Change the version byte.
						rawSig[0] = 2
						sig := base64.URLEncoding.EncodeToString(rawSig)
						err = verify(msg, sig, invalidSecret)
						if !errors.As(err, &e) {
							t.Errorf("Expected VerifyError, got %v", err)
						}
						if e.Reason != VerifyErrorReasonUnknownVersion {
							t.Errorf("Expected VerifyError with reason unknown version, got %v", e)
						}
					},
				)
			},
		)
	}
}

func BenchmarkV1(b *testing.B) {
	msg := []byte("Hello, world!")
	secret := secrets.VersionedSecret{Current: secrets.Secret("hunter2")}
	expiration := time.Now().Add(time.Hour * 24)

	var sig string
	var err error

	b.Run(
		"sign",
		func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				sig, err = V1.Sign(SignArgs{
					Message:   msg,
					Secret:    secret,
					ExpiresAt: expiration,
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		},
	)

	b.Run(
		"verify",
		func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err = V1.Verify(msg, sig, secret)
				if err != nil {
					b.Fatal(err)
				}
			}
		},
	)

	rotated := secrets.VersionedSecret{
		Current:  secrets.Secret("hunter0"),
		Previous: secrets.Secret("hunter1"),
		Next:     secrets.Secret("hunter2"),
	}

	b.Run(
		"verify-3keys",
		func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err = V1.Verify(msg, sig, rotated)
				if err != nil {
					b.Fatal(err)
				}
			}
		},
	)
}
