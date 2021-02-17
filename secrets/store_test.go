package secrets_test

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
)

const specificationExample = `
{
	"secrets": {
		"secret/myservice/external-account-key": {
			"type": "versioned",
			"current": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU=",
			"previous": "aHVudGVyMg=="
		},
		"secret/myservice/some-api-key": {
			"type": "simple",
			"value": "Y2RvVXhNMVdsTXJma3BDaHRGZ0dPYkVGSg==",
			"encoding": "base64"
		},
		"secret/myservice/some-database-credentials": {
			"type": "credential",
			"username": "spez",
			"password": "hunter2"
		}
	},
	"vault": {
		"url": "vault.reddit.ue1.snooguts.net",
		"token": "17213328-36d4-11e7-8459-525400f56d04"
	}
}`

func TestGetSimpleSecret(t *testing.T) {
	dir, err := os.MkdirTemp("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := os.CreateTemp(dir, "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write([]byte(specificationExample))
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		key           string
		expectedError error
		expected      secrets.SimpleSecret
	}{
		{
			name:     "specification example",
			key:      "secret/myservice/some-api-key",
			expected: secrets.SimpleSecret{Value: secrets.Secret("cdoUxM1WlMrfkpChtFgGObEFJ")},
		},
		{
			name:          "missing key",
			key:           "spez",
			expectedError: secrets.SecretNotFoundError("spez"),
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				store, err := secrets.NewStore(context.Background(), tmpPath, log.TestWrapper(t))
				if err != nil {
					t.Fatal(err)
				}
				defer store.Close()

				secret, err := store.GetSimpleSecret(tt.key)
				if tt.expectedError == nil && err != nil {
					t.Fatal(err)
				}
				if tt.expectedError != nil && err.Error() != tt.expectedError.Error() {
					t.Fatalf("expected error %v, actual: %v", tt.expectedError, err)
				}
				if !reflect.DeepEqual(secret, tt.expected) {
					t.Fatalf("expected %+v, actual: %+v", tt.expected, secret)
				}
			},
		)
	}
}

func TestGetVersionedSecret(t *testing.T) {
	dir, err := os.MkdirTemp("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := os.CreateTemp(dir, "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write([]byte(specificationExample))
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		key           string
		expectedError error
		expected      secrets.VersionedSecret
	}{
		{
			name: "specification example",
			key:  "secret/myservice/external-account-key",
			expected: secrets.VersionedSecret{
				Current:  secrets.Secret("YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU="),
				Previous: secrets.Secret("aHVudGVyMg=="),
			},
		},
		{
			name:          "missing key",
			key:           "spez",
			expectedError: secrets.SecretNotFoundError("spez"),
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				store, err := secrets.NewStore(context.Background(), tmpPath, log.TestWrapper(t))
				if err != nil {
					t.Fatal(err)
				}
				defer store.Close()

				secret, err := store.GetVersionedSecret(tt.key)
				if tt.expectedError == nil && err != nil {
					t.Fatal(err)
				}
				if tt.expectedError != nil && err.Error() != tt.expectedError.Error() {
					t.Fatalf("expected error %v, actual: %v", tt.expectedError, err)
				}
				if !reflect.DeepEqual(secret, tt.expected) {
					t.Fatalf("expected %+v, actual: %+v", tt.expected, secret)
				}
			},
		)
	}
}

func TestGetCredentialSecret(t *testing.T) {
	dir, err := os.MkdirTemp("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := os.CreateTemp(dir, "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write([]byte(specificationExample))
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		key           string
		expectedError error
		expected      secrets.CredentialSecret
	}{
		{
			name: "specification example",
			key:  "secret/myservice/some-database-credentials",
			expected: secrets.CredentialSecret{
				Username: "spez",
				Password: "hunter2",
			},
		},
		{
			name:          "missing key",
			key:           "spez",
			expectedError: secrets.SecretNotFoundError("spez"),
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				store, err := secrets.NewStore(context.Background(), tmpPath, log.TestWrapper(t))
				if err != nil {
					t.Fatal(err)
				}
				defer store.Close()

				secret, err := store.GetCredentialSecret(tt.key)
				if tt.expectedError == nil && err != nil {
					t.Fatal(err)
				}
				if tt.expectedError != nil && err.Error() != tt.expectedError.Error() {
					t.Fatalf("expected error %v, actual: %v", tt.expectedError, err)
				}
				if !reflect.DeepEqual(secret, tt.expected) {
					t.Fatalf("expected %+v, actual: %+v", tt.expected, secret)
				}
			},
		)
	}
}

func TestSecretFileIsUpdated(t *testing.T) {
	dir, err := os.MkdirTemp("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := os.CreateTemp(dir, "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write([]byte(specificationExample))
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := secrets.NewStore(context.Background(), tmpPath, log.TestWrapper(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	secret, err := store.GetSimpleSecret("secret/myservice/some-api-key")
	if err != nil {
		t.Fatal(err)
	}
	expected := "cdoUxM1WlMrfkpChtFgGObEFJ"
	if string(secret.Value) != expected {
		t.Fatalf("expected secret to be %s, actual: %s", expected, secret.Value)
	}

	updated := `{
		"secrets": {
			"secret/myservice/some-api-key": {
				"type": "simple",
				"value": "dXBkYXRlZCBzZWNyZXQ=",
				"encoding": "base64"
			}
		}
	}`

	tmpFile, err = os.CreateTemp(dir, "secrets2.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath2 := tmpFile.Name()
	tmpFile.Write([]byte(updated))
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmpPath2, tmpPath); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond * 100)

	secret, err = store.GetSimpleSecret("secret/myservice/some-api-key")
	if err != nil {
		t.Fatal(err)
	}
	expected = "updated secret"
	if string(secret.Value) != expected {
		t.Fatalf("expected secret to be %s, actual: %s", expected, secret.Value)
	}
}

type mockMiddleware struct {
	tb    testing.TB
	calls int

	simpleKey  string
	simpleData secrets.SimpleSecret

	versionedKey  string
	versionedData secrets.VersionedSecret

	credentialKey  string
	credentialData secrets.CredentialSecret
}

func (m *mockMiddleware) middleware(next secrets.SecretHandlerFunc) secrets.SecretHandlerFunc {
	return func(sec *secrets.Secrets) {
		m.tb.Helper()
		m.calls++
		if m.simpleKey != "" {
			data, err := sec.GetSimpleSecret(m.simpleKey)
			if err != nil {
				m.tb.Errorf("failed to get SimpleSecret %q: %v", m.simpleKey, err)
			}
			if !reflect.DeepEqual(data, m.simpleData) {
				m.tb.Errorf(
					"expected SimpleSecret for %q: %+v, got %+v",
					m.simpleKey,
					m.simpleData,
					data,
				)
			}
		}
		if m.versionedKey != "" {
			data, err := sec.GetVersionedSecret(m.versionedKey)
			if err != nil {
				m.tb.Errorf("failed to get VersionedSecret %q: %v", m.versionedKey, err)
			}
			if !reflect.DeepEqual(data, m.versionedData) {
				m.tb.Errorf(
					"expected VersionedSecret for %q: %+v, got %+v",
					m.versionedKey,
					m.versionedData,
					data,
				)
			}
		}
		if m.credentialKey != "" {
			data, err := sec.GetCredentialSecret(m.credentialKey)
			if err != nil {
				m.tb.Errorf("failed to get CredentialSecret %q: %v", m.credentialKey, err)
			}
			if !reflect.DeepEqual(data, m.credentialData) {
				m.tb.Errorf(
					"expected CredentialSecret for %q: %+v, got %+v",
					m.credentialKey,
					m.credentialData,
					data,
				)
			}
		}
		next(sec)
	}
}

func TestNewStoreMiddleware(t *testing.T) {
	dir, err := os.MkdirTemp("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := os.CreateTemp(dir, "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	defer tmpFile.Close()
	tmpFile.Write([]byte(specificationExample))

	const expectedMiddlewareCalls = 2
	m := mockMiddleware{
		tb: t,

		simpleKey: "secret/myservice/some-api-key",
		simpleData: secrets.SimpleSecret{
			Value: secrets.Secret("cdoUxM1WlMrfkpChtFgGObEFJ"),
		},

		versionedKey: "secret/myservice/external-account-key",
		versionedData: secrets.VersionedSecret{
			Current:  secrets.Secret("YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU="),
			Previous: secrets.Secret("aHVudGVyMg=="),
		},

		credentialKey: "secret/myservice/some-database-credentials",
		credentialData: secrets.CredentialSecret{
			Username: "spez",
			Password: "hunter2",
		},
	}

	store, err := secrets.NewStore(context.Background(), tmpFile.Name(), log.TestWrapper(t), m.middleware, m.middleware)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if m.calls != expectedMiddlewareCalls {
		t.Errorf("expecting %d calls, got %d instead", expectedMiddlewareCalls, m.calls)
	}
}

func TestAddMiddleware(t *testing.T) {
	dir, err := os.MkdirTemp("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := os.CreateTemp(dir, "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	defer tmpFile.Close()
	tmpFile.Write([]byte(specificationExample))

	initial := mockMiddleware{
		tb: t,

		simpleKey: "secret/myservice/some-api-key",
		simpleData: secrets.SimpleSecret{
			Value: secrets.Secret("cdoUxM1WlMrfkpChtFgGObEFJ"),
		},

		versionedKey: "secret/myservice/external-account-key",
		versionedData: secrets.VersionedSecret{
			Current:  secrets.Secret("YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU="),
			Previous: secrets.Secret("aHVudGVyMg=="),
		},

		credentialKey: "secret/myservice/some-database-credentials",
		credentialData: secrets.CredentialSecret{
			Username: "spez",
			Password: "hunter2",
		},
	}

	store, err := secrets.NewStore(context.Background(), tmpFile.Name(), log.TestWrapper(t), initial.middleware)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	const expectedAdditionalCalls = 2
	additional := initial
	additional.calls = 0
	store.AddMiddlewares(additional.middleware, additional.middleware)
	if expectedAdditionalCalls != additional.calls {
		t.Errorf(
			"expecting %d calls to additional middleware, got %d instead",
			expectedAdditionalCalls,
			additional.calls,
		)
	}
}
