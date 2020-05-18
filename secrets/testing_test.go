package secrets_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/secrets"
)

func TestNewTestSecrets(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	const (
		bar  = "bar"
		fizz = "fizz"
		foo  = "foo"

		username = "username"
		password = "password"
	)
	b64Foo := base64.StdEncoding.EncodeToString([]byte(foo))

	type expectation struct {
		simple     map[string]secrets.SimpleSecret
		versioned  map[string]secrets.VersionedSecret
		credential map[string]secrets.CredentialSecret
	}

	cases := []struct {
		name     string
		raw      map[string]secrets.GenericSecret
		expected expectation
	}{
		{
			name: "default",
			raw:  make(map[string]secrets.GenericSecret),
			expected: expectation{
				versioned: map[string]secrets.VersionedSecret{
					secrets.JWTPubKeyPath: {
						Current: secrets.Secret(secrets.TestJWTPubKeySecret.Current),
					},
				},
			},
		},
		{
			name: "set-edge-context",
			raw: map[string]secrets.GenericSecret{
				secrets.JWTPubKeyPath: {
					Type:    "versioned",
					Current: foo,
				},
			},
			expected: expectation{
				versioned: map[string]secrets.VersionedSecret{
					secrets.JWTPubKeyPath: {
						Current: secrets.Secret(foo),
					},
				},
			},
		},
		{
			name: "all",
			raw: map[string]secrets.GenericSecret{
				"secret/simple/identity": {
					Type:     "simple",
					Value:    foo,
					Encoding: secrets.IdentityEncoding,
				},
				"secret/simple/base64": {
					Type:     "simple",
					Value:    b64Foo,
					Encoding: secrets.Base64Encoding,
				},
				"secret/versioned": {
					Type:     "versioned",
					Current:  foo,
					Previous: bar,
					Next:     fizz,
				},
				"secret/credential": {
					Type:     "credential",
					Username: username,
					Password: password,
				},
			},
			expected: expectation{
				simple: map[string]secrets.SimpleSecret{
					"secret/simple/identity": {
						Value: secrets.Secret(foo),
					},
					"secret/simple/base64": {
						Value: secrets.Secret(foo),
					},
				},
				versioned: map[string]secrets.VersionedSecret{
					secrets.JWTPubKeyPath: {
						Current: secrets.Secret(secrets.TestJWTPubKeySecret.Current),
					},
					"secret/versioned": {
						Current:  secrets.Secret(foo),
						Previous: secrets.Secret(bar),
						Next:     secrets.Secret(fizz),
					},
				},
				credential: map[string]secrets.CredentialSecret{
					"secret/credential": {
						Username: username,
						Password: password,
					},
				},
			},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				store, _, err := secrets.NewTestSecrets(ctx, c.raw)
				if err != nil {
					t.Fatal(err)
				}

				t.Run(
					"simple",
					func(t *testing.T) {
						for path, expected := range c.expected.simple {
							secret, err := store.GetSimpleSecret(path)
							if err != nil {
								t.Error(err)
								continue
							}

							if !bytes.Equal(secret.Value, expected.Value) {
								t.Errorf(
									"secret.value mismatch, expected %q, got %q",
									expected.Value,
									secret.Value,
								)
							}
						}
					},
				)

				t.Run(
					"versioned",
					func(t *testing.T) {
						for path, expected := range c.expected.versioned {
							secret, err := store.GetVersionedSecret(path)
							if err != nil {
								t.Error(err)
								continue
							}

							allExpected := expected.GetAll()
							for i, val := range secret.GetAll() {
								if !bytes.Equal(val, allExpected[i]) {
									t.Errorf(
										"secret[%d] mismatch, expected %q, got %q",
										i,
										allExpected[i],
										val,
									)
								}
							}
						}
					},
				)

				t.Run(
					"credential",
					func(t *testing.T) {
						for path, expected := range c.expected.credential {
							secret, err := store.GetCredentialSecret(path)
							if err != nil {
								t.Error(err)
								continue
							}

							if strings.Compare(secret.Username, expected.Username) != 0 {
								t.Errorf(
									"secret.username mismatch, expected %q, got %q",
									expected.Username,
									secret.Username,
								)
							}
							if strings.Compare(secret.Password, expected.Password) != 0 {
								t.Errorf(
									"secret.password mismatch, expected %q, got %q",
									expected.Password,
									secret.Password,
								)
							}
						}
					},
				)
			},
		)
	}
}

func TestUpdateTestSecrets(t *testing.T) {
	t.Parallel()

	const (
		foo  = "foo"
		path = "secret/simple/test"
	)

	ctx := context.Background()
	store, fw, err := secrets.NewTestSecrets(
		ctx,
		make(map[string]secrets.GenericSecret),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.GetSimpleSecret(path); err == nil {
		t.Fatalf("mysterious secret at %q", path)
	}

	raw := map[string]secrets.GenericSecret{
		path: {
			Type:  "simple",
			Value: foo,
		},
	}
	if err = secrets.UpdateTestSecrets(fw, raw); err != nil {
		t.Fatal(err)
	}

	secret, err := store.GetSimpleSecret(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(secret.Value, []byte(foo)) {
		t.Errorf(
			"secret.value mismatch, expected %q, got %q",
			[]byte(foo),
			secret.Value,
		)
	}
}
