package edgecontext_test

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"

	"github.com/reddit/baseplate.go/httpbp"

	"github.com/reddit/baseplate.go/edgecontext"
)

const secretsFile = `{
	"secrets": {
		"secret/http/edge-context-signature": {
			"type": "versioned",
			"current": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU=",
			"previous": "aHVudGVyMg==",
			"encoding": "base64"
		},
		"secret/http/span-signature": {
			"type": "versioned",
			"current": "Y2RvVXhNMVdsTXJma3BDaHRGZ0dPYkVGSg==",
			"encoding": "base64"
		}
	},
	"vault": {
		"url": "vault.reddit.ue1.snooguts.net",
		"token": "17213328-36d4-11e7-8459-525400f56d04"
	}
}`

func TestAttachHTTPHeader(t *testing.T) {
	t.Parallel()

	e, err := edgecontext.New(
		context.Background(),
		edgecontext.NewArgs{
			LoID:          expectedLoID,
			LoIDCreatedAt: expectedCookieTime,
			SessionID:     expectedSessionID,
			AuthToken:     validToken,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"AttachHTTPHeader",
		func(t *testing.T) {
			t.Parallel()

			headers := make(http.Header)
			e.AttachHTTPHeader(headers)
			h := headers.Get(httpbp.EdgeContextHeader)
			if h == "" {
				t.Fatal("Header was not attached.")
			}
			ctx := httpbp.SetHeader(context.Background(), httpbp.EdgeContextContextKey, h)
			ec, err := edgecontext.FromHTTPContext(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(e, ec) {
				t.Fatalf("Expected %#v, got %#v", e, ec)
			}
		},
	)

	t.Run(
		"AttachSignedHTTPHeader",
		func(t *testing.T) {
			t.Parallel()

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
			if _, err = tmpFile.Write([]byte(secretsFile)); err != nil {
				t.Fatal(err)
			}
			if err := tmpFile.Close(); err != nil {
				t.Fatal(err)
			}

			store, err := secrets.NewStore(context.Background(), tmpPath, log.TestWrapper(t))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()

			handler := httpbp.NewTrustHeaderSignature(httpbp.TrustHeaderSignatureArgs{
				SecretsStore:          store,
				EdgeContextSecretPath: "secret/http/edge-context-signature",
				SpanSecretPath:        "secret/http/span-signature",
			})

			signer := edgecontext.HeaderTrustHandlerSigner(handler, time.Minute)

			headers := make(http.Header)
			if err = e.AttachSignedHTTPHeader(headers, signer); err != nil {
				t.Fatal(err)
			}

			h := headers.Get(httpbp.EdgeContextHeader)
			if h == "" {
				t.Fatal("Header was not attached.")
			}

			ctx := httpbp.SetHeader(context.Background(), httpbp.EdgeContextContextKey, h)
			ec, err := edgecontext.FromHTTPContext(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(e, ec) {
				t.Fatalf("Expected %#v, got %#v", e, ec)
			}

			sig := headers.Get(httpbp.EdgeContextSignatureHeader)
			if sig == "" {
				t.Fatal("Signature was not attached.")
			}

			r := &http.Request{Header: headers}
			if !handler.TrustEdgeContext(r) {
				t.Fatal("Signature check failed")
			}
		},
	)
}
