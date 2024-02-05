package secrets

import (
	"bytes"
	"reflect"
	"testing"
)

func TestNewSecrets(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expected      *Secrets
		expectedError error
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
			expected: &Secrets{
				simpleSecrets: map[string]SimpleSecret{
					"secret/myservice/some-api-key": {
						Value: Secret("cdoUxM1WlMrfkpChtFgGObEFJ"),
					},
				},
				versionedSecrets: map[string]VersionedSecret{
					"secret/myservice/external-account-key": {
						Current:  Secret("YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU="),
						Previous: Secret("aHVudGVyMg=="),
					},
				},
				credentialSecrets: map[string]CredentialSecret{
					"secret/myservice/some-database-credentials": {
						Username: "spez",
						Password: "hunter2",
					},
				},
				vault: Vault{
					URL:   "vault.reddit.ue1.snooguts.net",
					Token: "17213328-36d4-11e7-8459-525400f56d04",
				},
			},
		},
		{
			name:  "empty",
			input: `{}`,
			expected: &Secrets{
				simpleSecrets:     make(map[string]SimpleSecret),
				versionedSecrets:  make(map[string]VersionedSecret),
				credentialSecrets: make(map[string]CredentialSecret),
				vault: Vault{
					URL:   "",
					Token: "",
				},
			},
		},
		{
			name: "too many fields",
			input: `
					{
						"secrets": {
							"secret/myservice/some-api-key": {
								"type": "simple",
								"value": "hunter2",
								"current": "hunter2"
							}
						},
						"vault": {
							"url": "vault.reddit.ue1.snooguts.net",
							"token": "17213328-36d4-11e7-8459-525400f56d04"
						}
					}
			`,
			expectedError: TooManyFieldsError{
				SecretType: SimpleType,
				Key:        "secret/myservice/some-api-key",
			},
		},
	}
	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := bytes.NewBuffer([]byte(tt.input))
			secrets, err := NewSecrets(buf)
			if tt.expectedError == nil && err != nil {
				t.Fatal(err)
			}
			if tt.expectedError != nil && err.Error() != tt.expectedError.Error() {
				t.Fatalf("expected error %v, actual: %v", tt.expectedError, err)
			}
			if !reflect.DeepEqual(secrets, tt.expected) {
				t.Fatalf("expected %v, actual: %v", tt.expected, secrets)
			}
		})
	}
}

func TestSecretsWrongType(t *testing.T) {
	rawSecrets := `
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
	`

	tests := []struct {
		name          string
		input         string
		function      func(*Secrets) (interface{}, error)
		expectedError error
	}{
		{
			name:  "Simple vs Versioned",
			input: rawSecrets,
			function: func(s *Secrets) (interface{}, error) {
				return s.GetSimpleSecret("secret/myservice/external-account-key")
			},
			expectedError: SecretWrongTypeError{
				Path:         "secret/myservice/external-account-key",
				DeclaredType: "simple",
				CorrectType:  "versioned",
			},
		},
		{
			name:  "Versioned vs Simple",
			input: rawSecrets,
			function: func(s *Secrets) (interface{}, error) {
				return s.GetVersionedSecret("secret/myservice/some-api-key")
			},
			expectedError: SecretWrongTypeError{
				Path:         "secret/myservice/some-api-key",
				DeclaredType: "versioned",
				CorrectType:  "simple",
			},
		},
		{
			name:  "Credential vs Simple",
			input: rawSecrets,
			function: func(s *Secrets) (interface{}, error) {
				return s.GetCredentialSecret("secret/myservice/some-api-key")
			},
			expectedError: SecretWrongTypeError{
				Path:         "secret/myservice/some-api-key",
				DeclaredType: "credential",
				CorrectType:  "simple",
			},
		},
		{
			name:  "Simple vs Credential",
			input: rawSecrets,
			function: func(s *Secrets) (interface{}, error) {
				return s.GetSimpleSecret("secret/myservice/some-database-credentials")
			},
			expectedError: SecretWrongTypeError{
				Path:         "secret/myservice/some-database-credentials",
				DeclaredType: "simple",
				CorrectType:  "credential",
			},
		},
		{
			name:  "Versioned vs Credential",
			input: rawSecrets,
			function: func(s *Secrets) (interface{}, error) {
				return s.GetVersionedSecret("secret/myservice/some-database-credentials")
			},
			expectedError: SecretWrongTypeError{
				Path:         "secret/myservice/some-database-credentials",
				DeclaredType: "versioned",
				CorrectType:  "credential",
			},
		},
		{
			name:  "Credential vs Versioned",
			input: rawSecrets,
			function: func(s *Secrets) (interface{}, error) {
				return s.GetCredentialSecret("secret/myservice/external-account-key")
			},
			expectedError: SecretWrongTypeError{
				Path:         "secret/myservice/external-account-key",
				DeclaredType: "credential",
				CorrectType:  "versioned",
			},
		},
	}
	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := bytes.NewBuffer([]byte(tt.input))
			secrets, err := NewSecrets(buf)
			if tt.expectedError == nil && err != nil {
				t.Fatal(err)
			}
			_, err = tt.function(secrets)
			if tt.expectedError != nil && err.Error() != tt.expectedError.Error() {
				t.Fatalf("expected error %v, actual: %v", tt.expectedError, err)
			}
		})
	}
}
