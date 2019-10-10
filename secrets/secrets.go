package secrets

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	simpleSecret     = "simple"
	versionedSecret  = "versioned"
	credentialSecret = "credential"
)

// Secrets allows to access secrets based on their different type.
type Secrets struct {
	simpleSecrets     map[string]SimpleSecret
	versionedSecrets  map[string]VersionedSecret
	credentialSecrets map[string]CredentialSecret
	vault             Vault
}

// SimpleSecret represent basic string secrets.
type SimpleSecret struct {
	Value string
}

// Returns a new instance of SimpleSecret based on a
// GenericSecret from Document. If there is an encoding specified the
// raw secret will be decoded prior.
func newSimpleSecret(secret *GenericSecret) (SimpleSecret, error) {
	var result SimpleSecret
	value := secret.Value
	value, err := secret.Encoding.decodeValue(value)
	if err != nil {
		return result, err
	}
	return SimpleSecret{
		Value: value,
	}, nil
}

// VersionedSecret represent secrets like signing keys that can be rotated
// gracefully.
//
// The current property contains the active version of a secret. This should be
// used for any actions that generate new cryptographic data (e.g. signing a
// token).
//
// The previous and next fields contain old and not-yet-active versions of the
// secret respectively. These MAY be used by applications to give a grace
// period for cryptographic tokens generated during a rotation, but SHOULD NOT
// be used to generate new cryptographic tokens.
type VersionedSecret struct {
	Current  string
	Previous string
	Next     string
}

// Returns a new instance of VersionedSecret based on a
// GenericSecret from Document. If there is an encoding specified the
// raw secrets will be decoded prior.
func newVersionedSecret(secret *GenericSecret) (VersionedSecret, error) {
	var result VersionedSecret

	current := secret.Current
	previous := secret.Previous
	next := secret.Next

	currentSecret, err := secret.Encoding.decodeValue(current)
	if err != nil {
		return result, err
	}
	previousSecret, err := secret.Encoding.decodeValue(previous)
	if err != nil {
		return result, err
	}
	nextSecret, err := secret.Encoding.decodeValue(next)
	if err != nil {
		return result, err
	}
	return VersionedSecret{
		Current:  currentSecret,
		Previous: previousSecret,
		Next:     nextSecret,
	}, nil
}

// GetAll returns all versions that are not empty in the following order:
// current, previous, next.
func (v *VersionedSecret) GetAll() []string {
	allVersions := []string{v.Current}
	if v.Previous != "" {
		allVersions = append(allVersions, v.Previous)
	}
	if v.Next != "" {
		allVersions = append(allVersions, v.Next)
	}
	return allVersions
}

// CredentialSecret represent represent username/password pairs as a single
// secret in vault. Note that usernames are not generally considered secret,
// but they are tied to passwords.
type CredentialSecret struct {
	Username string
	Password string
}

// NewCredentialSecret returns a new instance of CredentialSecret based on a
// GenericSecret from Document.
func newCredentialSecret(secret *GenericSecret) (CredentialSecret, error) {
	return CredentialSecret{
		Username: secret.Username,
		Password: secret.Password,
	}, nil
}

// Document represents the raw parsed entity of a Secrets JSON and is
// not meant to be used other than instantiating Secrets.
type Document struct {
	Secrets map[string]GenericSecret `json:"secrets"`
	Vault   Vault                    `json:"vault"`
}

// ValidationErrors is a set of errors found while parsing the Secrets JSON.
type ValidationErrors []error

// Error implements the error interface.
func (v ValidationErrors) Error() string {
	errorStrings := make([]string, len(v))
	for i, err := range v {
		errorStrings[i] = err.Error()
	}
	return strings.Join(errorStrings, "\n")
}

// Validate checks the Document for any errors that violate the
// Baseplate specification.
func (s *Document) Validate() error {
	var errs ValidationErrors
	for key, value := range s.Secrets {
		if value.Type == simpleSecret && notOnlySimpleSecret(value) {
			errs = append(errs, tooManyFieldsError(simpleSecret, key))
		}
		if value.Type == versionedSecret && notOnlyVersionedSecret(value) {
			errs = append(errs, tooManyFieldsError(versionedSecret, key))
		}
		if value.Type == credentialSecret && notOnlyCredentialSecret(value) {
			errs = append(errs, tooManyFieldsError(credentialSecret, key))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func tooManyFieldsError(secretType, key string) error {
	return fmt.Errorf("expected %s secret but other fields were present for %s", secretType, key)
}

func notOnlySimpleSecret(secret GenericSecret) bool {
	return secret.Current != "" || secret.Previous != "" || secret.Next != "" || secret.Username != "" || secret.Password != ""
}

func notOnlyVersionedSecret(secret GenericSecret) bool {
	return secret.Value != "" || secret.Username != "" || secret.Password != ""
}

func notOnlyCredentialSecret(secret GenericSecret) bool {
	return secret.Value != "" || secret.Current != "" || secret.Previous != "" || secret.Next != ""
}

// GenericSecret is a placeholder to fit all types of secrets when parsing the
// Secret JSON before processing them into their more typed equivalents.
type GenericSecret struct {
	Type     string   `json:"type"`
	Value    string   `json:"value"`
	Encoding encoding `json:"encoding"`

	Current  string `json:"current"`
	Previous string `json:"previous"`
	Next     string `json:"next"`

	Username string `json:"username"`
	Password string `json:"password"`
}

// Vault provides authentication credentials so that applications can directly
// connect to Vault for more complicated use cases.
type Vault struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// NewSecrets parses and validates the secret JSON provided by the reader.
func NewSecrets(r io.Reader) (*Secrets, error) {
	var secretsDocument Document
	err := json.NewDecoder(r).Decode(&secretsDocument)
	if err != nil {
		return nil, err
	}

	err = secretsDocument.Validate()
	if err != nil {
		return nil, err
	}
	secrets := &Secrets{
		simpleSecrets:     make(map[string]SimpleSecret),
		versionedSecrets:  make(map[string]VersionedSecret),
		credentialSecrets: make(map[string]CredentialSecret),
		vault:             secretsDocument.Vault,
	}
	for key, secret := range secretsDocument.Secrets {
		switch secret.Type {
		case "simple":
			simple, err := newSimpleSecret(&secret)
			if err != nil {
				return nil, err
			}
			secrets.simpleSecrets[key] = simple
		case "versioned":
			versioned, err := newVersionedSecret(&secret)
			if err != nil {
				return nil, err
			}
			secrets.versionedSecrets[key] = versioned
		case "credential":
			credential, err := newCredentialSecret(&secret)
			if err != nil {
				return nil, err
			}
			secrets.credentialSecrets[key] = credential
		default:
			return nil, fmt.Errorf("encountered unknown secret type %s", secret.Type)
		}
	}
	return secrets, nil
}

// Encoding represents the encoding used to encode the secrets.
type encoding int

const (
	identityEncoding encoding = iota
	base64Encoding
)

func (e *encoding) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "identity", "":
		*e = identityEncoding
	case "base64":
		*e = base64Encoding
	default:
		return errors.New("invalid encoding, expected identity, base64 or empty")
	}
	return nil
}

func (e encoding) decodeValue(value string) (string, error) {
	switch e {
	case identityEncoding:
		return value, nil
	default:
		data, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}
