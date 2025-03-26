package secrets

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"reflect"
	"testing"
	"time"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/util/fileutil"

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

	dir := t.TempDir()

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				tmpFile, err := os.CreateTemp(dir, "secrets.json")
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

func TestDirRotation(t *testing.T) {
	const (
		delay = 50 * time.Millisecond
		sleep = delay * 5
	)
	const (
		key1 = "secrets/foo"
		key2 = "secrets/bar"

		apiKey = `
{
  "request_id": "1afc3036-2282-d483-c2d4-6d483efdf16c",
  "lease_id": "",
  "lease_duration": 2764800,
  "renewable": false,
  "data": {
    "type": "simple",
    "value": "Y2RvVXhNMVdsTXJma3BDaHRGZ0dPYkVGSg==",
    "encoding": "base64"
  },
  "warnings": null
}
`
	)
	var wantSecret = SimpleSecret{Value: Secret("cdoUxM1WlMrfkpChtFgGObEFJ")}

	dir := t.TempDir()
	writer, err := fileutil.NewAtomicWriter(dir, "")
	if err != nil {
		t.Fatalf("Failed to create k8s atomic writer: %v", err)
	}
	content := fileutil.FileProjection{
		Data: []byte(apiKey),
		Mode: 0777,
	}
	if err := writer.Write(map[string]fileutil.FileProjection{
		key1: content,
	}); err != nil {
		t.Fatalf("Failed to write initial payload: %v", err)
	}

	store, err := newStore(context.Background(), delay, dir, log.TestWrapper(t))
	if err != nil {
		t.Fatalf("Failed to create secrets store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	t.Run("initial-payload", func(t *testing.T) {
		const (
			correctKey = key1
			wrongKey   = key2
		)
		_, err := store.GetSimpleSecret(wrongKey)
		if err == nil {
			t.Errorf("Expected error when getting %q, got nil", wrongKey)
		}

		secret, err := store.GetSimpleSecret(correctKey)
		if err != nil {
			t.Fatalf("Expected no error when getting %q, got %v", correctKey, err)
		}
		if !reflect.DeepEqual(secret, wantSecret) {
			t.Errorf("Got secret %+v, want %+v", secret, wantSecret)
		}
	})

	t.Run("rotated-payload", func(t *testing.T) {
		const (
			correctKey = key2
			wrongKey   = key1
		)
		if err := writer.Write(map[string]fileutil.FileProjection{
			correctKey: content,
		}); err != nil {
			t.Fatalf("Failed to write rotated payload: %v", err)
		}
		time.Sleep(sleep)

		_, err := store.GetSimpleSecret(wrongKey)
		if err == nil {
			t.Errorf("Expected error when getting %q, got nil", wrongKey)
		}

		secret, err := store.GetSimpleSecret(correctKey)
		if err != nil {
			t.Fatalf("Expected no error when getting %q, got %v", correctKey, err)
		}
		if !reflect.DeepEqual(secret, wantSecret) {
			t.Errorf("Got secret %+v, want %+v", secret, wantSecret)
		}
	})

	t.Run("rotate_back", func(t *testing.T) {
		const (
			correctKey = key1
			wrongKey   = key2
		)
		if err := writer.Write(map[string]fileutil.FileProjection{
			correctKey: content,
		}); err != nil {
			t.Fatalf("Failed to write initial payload: %v", err)
		}
		time.Sleep(sleep)

		_, err := store.GetSimpleSecret(wrongKey)
		if err == nil {
			t.Errorf("Expected error when getting %q, got nil", wrongKey)
		}

		secret, err := store.GetSimpleSecret(correctKey)
		if err != nil {
			t.Fatalf("Expected no error when getting %q, got %v", correctKey, err)
		}
		if !reflect.DeepEqual(secret, wantSecret) {
			t.Errorf("Got secret %+v, want %+v", secret, wantSecret)
		}
	})

	t.Run("rotated-payload-again", func(t *testing.T) {
		const (
			correctKey = key2
			wrongKey   = key1
		)
		if err := writer.Write(map[string]fileutil.FileProjection{
			correctKey: content,
		}); err != nil {
			t.Fatalf("Failed to write rotated payload: %v", err)
		}
		time.Sleep(sleep)

		_, err := store.GetSimpleSecret(wrongKey)
		if err == nil {
			t.Errorf("Expected error when getting %q, got nil", wrongKey)
		}

		secret, err := store.GetSimpleSecret(correctKey)
		if err != nil {
			t.Fatalf("Expected no error when getting %q, got %v", correctKey, err)
		}
		if !reflect.DeepEqual(secret, wantSecret) {
			t.Errorf("Got secret %+v, want %+v", secret, wantSecret)
		}
	})
}

func TestDirectoryError(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	t.Cleanup(cancel)
	store, err := NewStore(ctx, dir, log.NopWrapper)
	if err == nil {
		store.Close()
		t.Fatal("Expected NewStore to return an error on an empty directory, got nil")
	}
	t.Logf("NewStore returned error: %v", err)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Error is not %v", fs.ErrNotExist)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Error is not %v", context.Canceled)
	}
	if !errors.As(err, new(notCSIError)) {
		t.Error("Error is not of type notCSIError")
	}
}
