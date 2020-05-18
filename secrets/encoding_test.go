package secrets

import (
	"errors"
	"strings"
	"testing"
)

func TestEncoding(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		enc        Encoding
		marshalled string
		err        error
	}{
		{
			name:       "invalid",
			enc:        -1,
			marshalled: `"invalid"`,
			err:        ErrInvalidEncoding,
		},
		{
			name:       "identity",
			enc:        IdentityEncoding,
			marshalled: identityEncodingJSON,
		},
		{
			name:       "base64",
			enc:        Base64Encoding,
			marshalled: base64EncodingJSON,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			"c.name",
			func(t *testing.T) {
				t.Run(
					"MarshalJSON",
					func(t *testing.T) {
						b, err := c.enc.MarshalJSON()
						if !errors.Is(err, c.err) {
							t.Fatalf("error mismatch, expected %#v, got %#v", c.err, err)
						}
						if err != nil {
							return
						}

						marshalled := string(b)
						if strings.Compare(marshalled, c.marshalled) != 0 {
							t.Fatalf("value mismatch, expected %q, got %q", c.marshalled, marshalled)
						}
					},
				)

				t.Run(
					"UnmarshalJSON",
					func(t *testing.T) {
						var e Encoding
						err := (&e).UnmarshalJSON([]byte(c.marshalled))
						if !errors.Is(err, c.err) {
							t.Fatalf("error mismatch, expected %#v, got %#v", c.err, err)
						}
						if err != nil {
							return
						}

						if e != c.enc {
							t.Fatalf("encoding does not match, expected %v, got %v", c.enc, e)
						}
					},
				)
			},
		)
	}

	t.Run(
		"UnmarshalJSON/fallback",
		func(t *testing.T) {
			var e Encoding
			err := (&e).UnmarshalJSON([]byte(`""`))
			if err != nil {
				t.Fatal(err)
			}

			if e != IdentityEncoding {
				t.Fatalf(
					"encoding does not match, expected %v, got %v",
					IdentityEncoding,
					e,
				)
			}
		},
	)
}
