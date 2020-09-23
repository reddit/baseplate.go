package secrets

import (
	"errors"
	"fmt"
)

// ErrInvalidEncoding is the error returned by the parser when we got an invalid
// encoding in the secrets.json file.
var ErrInvalidEncoding = errors.New("secrets: invalid encoding, expected identity, base64 or empty")

// ErrEmptySecretKey is returned when the path for a secret is empty.
var ErrEmptySecretKey = errors.New("secrets: secret path cannot be empty")

// TooManyFieldsError is a type of errors could be returned by
// Document.Validate.
//
// Note that Document.Validate could also return a BatchError containing
// multiple TooManyFieldsError.
type TooManyFieldsError struct {
	Key        string
	SecretType string
}

func (e TooManyFieldsError) Error() string {
	return fmt.Sprintf(
		"secrets: expected %s secret but other fields were present for %s",
		e.SecretType,
		e.Key,
	)
}

// SecretNotFoundError is returned when the key for a secret is not present in
// the secret store.
type SecretNotFoundError string

func (path SecretNotFoundError) Error() string {
	return "secrets: no secret has been found for " + string(path)
}
