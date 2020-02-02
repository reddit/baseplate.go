package secrets

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/reddit/baseplate.go/log"
)

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
				defer store.Close()

				if store.watcher.Get() == nil {
					t.Fatal("expected secret store watcher to return secrets")
				}
			},
		)
	}
}
