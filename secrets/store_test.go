package secrets

import (
	"context"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"github.snooguts.net/reddit/baseplate.go/log"
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

	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := ioutil.TempFile("", "secrets.json")
			if err != nil {
				t.Fatal(err)
			}
			tmpFile.Write([]byte(tt.input))
			store, err := NewStore(context.Background(), tmpFile.Name(), log.TestWrapper(t))
			if err != nil {
				t.Fatal(err)
			}
			if store.watcher.Get() == nil {
				t.Fatal("expected secret store watcher to return secrets")
			}
		})
	}
}

func TestGetSimpleSecret(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Write([]byte(specificationExample))

	tests := []struct {
		name          string
		key           string
		expectedError error
		expected      SimpleSecret
	}{
		{
			name:     "specification example",
			key:      "secret/myservice/some-api-key",
			expected: SimpleSecret{Value: "cdoUxM1WlMrfkpChtFgGObEFJ"},
		},
		{
			name:          "missing key",
			key:           "spez",
			expectedError: ErrorSecretNotFound("spez"),
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewStore(context.Background(), tmpFile.Name(), log.TestWrapper(t))
			if err != nil {
				t.Fatal(err)
			}
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
		})
	}
}

func TestGetVersionedSecret(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Write([]byte(specificationExample))

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
				Current:  "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU=",
				Previous: "aHVudGVyMg==",
			},
		},
		{
			name:          "missing key",
			key:           "spez",
			expectedError: ErrorSecretNotFound("spez"),
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewStore(context.Background(), tmpFile.Name(), log.TestWrapper(t))
			if err != nil {
				t.Fatal(err)
			}
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
		})
	}
}

func TestGetCredentialSecret(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Write([]byte(specificationExample))

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
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewStore(context.Background(), tmpFile.Name(), log.TestWrapper(t))
			if err != nil {
				t.Fatal(err)
			}
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
		})
	}
}

func TestSecretFileIsUpdated(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Write([]byte(specificationExample))

	store, err := NewStore(context.Background(), tmpFile.Name(), log.TestWrapper(t))
	if err != nil {
		t.Fatal(err)
	}
	secret, err := store.GetSimpleSecret("secret/myservice/some-api-key")
	if err != nil {
		t.Fatal(err)
	}
	expected := "cdoUxM1WlMrfkpChtFgGObEFJ"
	if secret.Value != expected {
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

	tmpFile.Truncate(0)
	tmpFile.WriteAt([]byte(updated), 0)
	time.Sleep(time.Millisecond * 1000)

	secret, err = store.GetSimpleSecret("secret/myservice/some-api-key")
	if err != nil {
		t.Fatal(err)
	}
	expected = "updated secret"
	if secret.Value != expected {
		t.Fatalf("expected secret to be %s, actual: %s", expected, secret.Value)
	}
}
