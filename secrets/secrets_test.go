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
			expectedError: tooManyFieldsError(simpleSecret, "secret/myservice/some-api-key"),
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
