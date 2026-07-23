package secrets

import (
	"bytes"
	"encoding/json"
	"errors"
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
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tt.function(secrets); !errors.Is(err, tt.expectedError) {
				t.Fatalf("expected error %v, actual: %v", tt.expectedError, err)
			}
		})
	}
}

// TestCSIFile_UnmarshalJSON exercises CSIFile's KV v1/v2 envelope handling.
//
// Mirrors the matching test in github.snooguts.net/reddit-go/secretsbp's
// internal/vaultcsi package, adapted to assert on the decoded GenericSecret
// (which is what CSIFile exposes) rather than the raw payload bytes.
func TestCSIFile_UnmarshalJSON(t *testing.T) {
	// populatedSecret is the GenericSecret callers see when v1/v2 detection
	// works correctly: the user payload is decoded into our typed fields.
	populatedSecret := GenericSecret{
		Type:     "simple",
		Value:    "abc",
		Encoding: IdentityEncoding,
	}

	tests := []struct {
		name    string
		input   string
		want    GenericSecret
		wantErr bool
	}{
		{
			name: "KV v1 envelope: user payload decoded",
			input: `{
				"request_id": "req-1",
				"lease_id": "lease-1",
				"lease_duration": 3600,
				"renewable": true,
				"data": {"type": "simple", "value": "abc", "encoding": "identity"},
				"warnings": ["heads up"]
			}`,
			want: populatedSecret,
		},
		{
			name: "KV v2 envelope: inner data hoisted, metadata dropped",
			input: `{
				"request_id": "req-2",
				"lease_duration": 60,
				"data": {
					"data": {"type": "simple", "value": "abc", "encoding": "identity"},
					"metadata": {
						"version": 7,
						"created_time": "2024-01-01T00:00:00Z",
						"destroyed": false,
						"deletion_time": "",
						"custom_metadata": null
					}
				}
			}`,
			want: populatedSecret,
		},
		{
			name: "v2-shaped but metadata.version missing: treated as v1",
			input: `{"data": {
				"data": {"type": "simple", "value": "abc"},
				"metadata": {"created_time": "2024-01-01T00:00:00Z"}
			}}`,
			// Outer payload is decoded as GenericSecret; none of its fields
			// match "data" or "metadata", so the result is zero-valued. A
			// non-zero result here would mean v2 detection misfired.
			want: GenericSecret{},
		},
		{
			name: "v2-shaped but metadata.version is a string: treated as v1",
			input: `{"data": {
				"data": {"type": "simple", "value": "abc"},
				"metadata": {"version": "7", "created_time": "2024-01-01T00:00:00Z"}
			}}`,
			want: GenericSecret{},
		},
		{
			name: "v2-shaped but metadata.created_time missing: treated as v1",
			input: `{"data": {
				"data": {"type": "simple", "value": "abc"},
				"metadata": {"version": 1}
			}}`,
			want: GenericSecret{},
		},
		{
			name: "v2-shaped but metadata.created_time is empty string: treated as v1",
			input: `{"data": {
				"data": {"type": "simple", "value": "abc"},
				"metadata": {"version": 1, "created_time": ""}
			}}`,
			want: GenericSecret{},
		},
		{
			name: "v2-shaped but metadata.created_time is a number: treated as v1",
			input: `{"data": {
				"data": {"type": "simple", "value": "abc"},
				"metadata": {"version": 1, "created_time": 1700000000}
			}}`,
			want: GenericSecret{},
		},
		{
			name: "v2-shaped but metadata is not an object: treated as v1",
			input: `{"data": {
				"data": {"type": "simple", "value": "abc"},
				"metadata": "version 1"
			}}`,
			want: GenericSecret{},
		},
		{
			name: "v2 with negative metadata.version: still treated as v2",
			input: `{"data": {
				"data": {"type": "simple", "value": "abc", "encoding": "identity"},
				"metadata": {"version": -1, "created_time": "2024-01-01T00:00:00Z"}
			}}`,
			want: populatedSecret,
		},
		{
			name:    "data is a JSON string: error, no panic",
			input:   `{"data": "just-a-string"}`,
			wantErr: true,
		},
		{
			name:    "data is a JSON array: error, no panic",
			input:   `{"data": [1, 2, 3]}`,
			wantErr: true,
		},
		{
			name:    "data is a JSON number: error, no panic",
			input:   `{"data": 42}`,
			wantErr: true,
		},
		{
			name:  "no data field: zero secret, no error",
			input: `{"request_id": "req-3"}`,
			want:  GenericSecret{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got CSIFile
			err := json.Unmarshal([]byte(tt.input), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; secret=%+v", got.Secret)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got.Secret, tt.want) {
				t.Fatalf("Secret mismatch:\n got: %+v\nwant: %+v", got.Secret, tt.want)
			}
		})
	}
}
