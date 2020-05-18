package secrets

import (
	"encoding/base64"
	"encoding/json"
)

// Encoding represents the Encoding used to encode a secret.
type Encoding int

const (
	// IdentityEncoding indicates no encoding beyond JSON itself.
	IdentityEncoding Encoding = iota
	// Base64Encoding indicates that the secret is base64 encoded.
	Base64Encoding
)

const (
	identityEncodingJSON = `"identity"`
	identityEncodingStr  = "identity"

	base64EncodingJSON = `"base64"`
	base64EncodingStr  = "base64"
)

// MarshalJSON returns a JSON string representation of the encoding.
func (e Encoding) MarshalJSON() ([]byte, error) {
	switch e {
	case IdentityEncoding:
		return []byte(identityEncodingJSON), nil
	case Base64Encoding:
		return []byte(base64EncodingJSON), nil
	default:
		return nil, ErrInvalidEncoding
	}
}

// UnmarshalJSON unmarshals the given JSON data into an encoding.
func (e *Encoding) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case identityEncodingStr, "":
		*e = IdentityEncoding
	case base64EncodingStr:
		*e = Base64Encoding
	default:
		return ErrInvalidEncoding
	}
	return nil
}

func (e Encoding) decodeValue(value string) (Secret, error) {
	if value == "" {
		return nil, nil
	}
	switch e {
	case IdentityEncoding:
		return Secret(value), nil
	default:
		data, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, err
		}
		return Secret(data), nil
	}
}

var (
	_ json.Marshaler   = Encoding(0)
	_ json.Unmarshaler = (*Encoding)(nil)
)
