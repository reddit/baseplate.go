package secrets

import (
	"context"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/log"
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

func TestNewStore(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Secrets
	}{
		{
			name: "specification example",
			input: `
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
					}
			`,
		},
	}

	dir, err := ioutil.TempDir("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				tmpFile, err := ioutil.TempFile(dir, "secrets.json")
				if err != nil {
					t.Fatal(err)
				}
				tmpPath := tmpFile.Name()
				tmpFile.Write([]byte(tt.input))
				if err := tmpFile.Close(); err != nil {
					t.Fatal(err)
				}

				store, err := NewStore(context.Background(), tmpPath, log.TestWrapper(t))
				if err != nil {
					t.Fatal(err)
				}
				defer store.watcher.Stop()
				if store.watcher.Get() == nil {
					t.Fatal("expected secret store watcher to return secrets")
				}
			},
		)
	}
}

func TestNewStoreMiddleware(t *testing.T) {
	dir, err := ioutil.TempDir("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}

	tmpFile, err := ioutil.TempFile(dir, "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Write([]byte(specificationExample))

	var (
		expectedMiddlewareCalls = 2
		middlewareCall          int

		middleware = func(next SecretHandlerFunc) SecretHandlerFunc {
			return func(sec *Secrets) {
				middlewareCall++
				next(sec)
			}
		}
	)
	_, err = NewStore(context.Background(), tmpFile.Name(), log.TestWrapper(t), middleware, middleware)
	if err != nil {
		t.Fatal(err)
	}

	if middlewareCall != expectedMiddlewareCalls {
		t.Errorf("expecting %d calls, got %d instead", expectedMiddlewareCalls, middlewareCall)
	}
}

func TestGetSimpleSecret(t *testing.T) {
	dir, err := ioutil.TempDir("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := ioutil.TempFile(dir, "secrets.json")
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
		expected      SimpleSecret
	}{
		{
			name:     "specification example",
			key:      "secret/myservice/some-api-key",
			expected: SimpleSecret{Value: Secret("cdoUxM1WlMrfkpChtFgGObEFJ")},
		},
		{
			name:          "missing key",
			key:           "spez",
			expectedError: ErrorSecretNotFound("spez"),
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				store, err := NewStore(context.Background(), tmpPath, log.TestWrapper(t))
				if err != nil {
					t.Fatal(err)
				}
				defer store.watcher.Stop()
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
	dir, err := ioutil.TempDir("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := ioutil.TempFile(dir, "secrets.json")
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
		expected      VersionedSecret
	}{
		{
			name: "specification example",
			key:  "secret/myservice/external-account-key",
			expected: VersionedSecret{
				Current:  Secret("YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU="),
				Previous: Secret("aHVudGVyMg=="),
			},
		},
		{
			name:          "missing key",
			key:           "spez",
			expectedError: ErrorSecretNotFound("spez"),
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				store, err := NewStore(context.Background(), tmpPath, log.TestWrapper(t))
				if err != nil {
					t.Fatal(err)
				}
				defer store.watcher.Stop()
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
	dir, err := ioutil.TempDir("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := ioutil.TempFile(dir, "secrets.json")
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
		expected      CredentialSecret
	}{
		{
			name: "specification example",
			key:  "secret/myservice/some-database-credentials",
			expected: CredentialSecret{
				Username: "spez",
				Password: "hunter2",
			},
		},
		{
			name:          "missing key",
			key:           "spez",
			expectedError: ErrorSecretNotFound("spez"),
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				store, err := NewStore(context.Background(), tmpPath, log.TestWrapper(t))
				if err != nil {
					t.Fatal(err)
				}
				defer store.watcher.Stop()
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
	dir, err := ioutil.TempDir("", "secret_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := ioutil.TempFile(dir, "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write([]byte(specificationExample))
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(context.Background(), tmpPath, log.TestWrapper(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.watcher.Stop()
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

	tmpFile, err = ioutil.TempFile(dir, "secrets2.json")
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
	time.Sleep(time.Millisecond * 10)

	secret, err = store.GetSimpleSecret("secret/myservice/some-api-key")
	if err != nil {
		t.Fatal(err)
	}
	expected = "updated secret"
	if string(secret.Value) != expected {
		t.Fatalf("expected secret to be %s, actual: %s", expected, secret.Value)
	}
}
