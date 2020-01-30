package httpbp_test

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
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

func getHeaderSignatureTrustHandler(secretsStore *secrets.Store) httpbp.HeaderSignatureTrustHandler {
	return httpbp.NewHeaderSignatureTrustHandler(httpbp.HeaderSignatureTrustHandlerArgs{
		SecretsStore:          secretsStore,
		EdgeContextSecretPath: "secret/http/edge-context-signature",
		SpanSecretPath:        "secret/http/span-signature",
	})
}

func TestNeverTrustHeaders(t *testing.T) {
	t.Parallel()

	request := http.Request{Header: getHeaders()}
	ctx := httpbp.PopulateBaseplateRequestContext(
		httpbp.NeverTrustHeaders{},
	)(context.Background(), &request)

	cases := []struct {
		name     string
		key      httpbp.HeaderContextKey
		expected string
	}{
		{
			name:     "Trace ID",
			key:      httpbp.TraceIDContextKey,
			expected: "",
		},
		{
			name:     "Parent ID",
			key:      httpbp.ParentIDContextKey,
			expected: "",
		},
		{
			name:     "Span ID",
			key:      httpbp.SpanIDContextKey,
			expected: "",
		},
		{
			name:     "Flags",
			key:      httpbp.SpanFlagsContextKey,
			expected: "",
		},
		{
			name:     "Sampled",
			key:      httpbp.SpanSampledContextKey,
			expected: "",
		},
		{
			name:     "Edge Context",
			key:      httpbp.EdgeContextContextKey,
			expected: "",
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()
				header := httpbp.GetHeader(ctx, c.key)
				if header != c.expected {
					t.Errorf(
						"Expected %s to be %v, got %v",
						c.name,
						c.expected,
						header,
					)
				}
			},
		)
	}
}

func TestHeaderSignatureTrustHandlerSignAndVerify(t *testing.T) {
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
	tmpFile.Write([]byte(secretsFile))
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := secrets.NewStore(context.Background(), tmpPath, log.TestWrapper(t))
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"edge context",
		func(t *testing.T) {
			request := http.Request{Header: getHeaders()}
			trustHandler := getHeaderSignatureTrustHandler(store)
			signature, err := trustHandler.SignEdgeContextHeader(
				httpbp.NewEdgeContextHeaders(request.Header),
				time.Minute,
			)
			if err != nil {
				t.Fatalf("Got an unexpected error while trying to sign headers: %v", err)
			}

			ok, err := trustHandler.VerifyEdgeContextHeader(
				httpbp.NewEdgeContextHeaders(request.Header),
				signature,
			)
			if err != nil {
				t.Errorf(
					"Got an unexpected error while trying to verify signature: %v",
					err,
				)
			}
			if !ok {
				t.Errorf("Signature %v failed to verify", signature)
			}
		},
	)

	t.Run(
		"span",
		func(t *testing.T) {
			request := http.Request{Header: getHeaders()}
			trustHandler := getHeaderSignatureTrustHandler(store)
			signature, err := trustHandler.SignSpanHeaders(
				httpbp.NewSpanHeaders(request.Header),
				time.Minute,
			)
			if err != nil {
				t.Fatalf("Got an unexpected error while trying to sign headers: %v", err)
			}

			ok, err := trustHandler.VerifySpanHeaders(
				httpbp.NewSpanHeaders(request.Header),
				signature,
			)
			if err != nil {
				t.Errorf(
					"Got an unexpected error while trying to verify signature: %v",
					err,
				)
			}
			if !ok {
				t.Errorf("Signature %v failed to verify", signature)
			}
		},
	)
}

func TestHeaderSignatureTrustHandler(t *testing.T) {
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
	tmpFile.Write([]byte(secretsFile))
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := secrets.NewStore(context.Background(), tmpPath, log.TestWrapper(t))
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"no signature",
		func(t *testing.T) {
			request := http.Request{Header: getHeaders()}
			ctx := httpbp.PopulateBaseplateRequestContext(
				getHeaderSignatureTrustHandler(store),
			)(context.Background(), &request)

			cases := []struct {
				name     string
				key      httpbp.HeaderContextKey
				expected string
			}{
				{
					name:     "Trace ID",
					key:      httpbp.TraceIDContextKey,
					expected: "",
				},
				{
					name:     "Parent ID",
					key:      httpbp.ParentIDContextKey,
					expected: "",
				},
				{
					name:     "Span ID",
					key:      httpbp.SpanIDContextKey,
					expected: "",
				},
				{
					name:     "Flags",
					key:      httpbp.SpanFlagsContextKey,
					expected: "",
				},
				{
					name:     "Sampled",
					key:      httpbp.SpanSampledContextKey,
					expected: "",
				},
				{
					name:     "Edge Context",
					key:      httpbp.EdgeContextContextKey,
					expected: "",
				},
			}
			for _, _c := range cases {
				c := _c
				t.Run(
					c.name,
					func(t *testing.T) {
						header := httpbp.GetHeader(ctx, c.key)
						if header != c.expected {
							t.Errorf(
								"Expected %s to be %v, got %v",
								c.name,
								c.expected,
								header,
							)
						}
					},
				)
			}
		},
	)

	t.Run(
		"edge context signature",
		func(t *testing.T) {
			request := http.Request{Header: getHeaders()}
			trustHandler := getHeaderSignatureTrustHandler(store)
			signature, err := trustHandler.SignEdgeContextHeader(
				httpbp.NewEdgeContextHeaders(request.Header),
				time.Minute,
			)
			if err != nil {
				t.Fatalf("Got an unexpected error while trying to sign headers: %v", err)
			}

			request.Header.Add(
				httpbp.EdgeContextSignatureHeader,
				signature,
			)
			ctx := httpbp.PopulateBaseplateRequestContext(
				getHeaderSignatureTrustHandler(store),
			)(context.Background(), &request)

			cases := []struct {
				name     string
				key      httpbp.HeaderContextKey
				expected string
			}{
				{
					name:     "Trace ID",
					key:      httpbp.TraceIDContextKey,
					expected: "",
				},
				{
					name:     "Parent ID",
					key:      httpbp.ParentIDContextKey,
					expected: "",
				},
				{
					name:     "Span ID",
					key:      httpbp.SpanIDContextKey,
					expected: "",
				},
				{
					name:     "Flags",
					key:      httpbp.SpanFlagsContextKey,
					expected: "",
				},
				{
					name:     "Sampled",
					key:      httpbp.SpanSampledContextKey,
					expected: "",
				},
				{
					name:     "Edge Context",
					key:      httpbp.EdgeContextContextKey,
					expected: edgeContext,
				},
			}
			for _, _c := range cases {
				c := _c
				t.Run(
					c.name,
					func(t *testing.T) {
						header := httpbp.GetHeader(ctx, c.key)
						if header != c.expected {
							t.Errorf(
								"Expected %s to be %v, got %v",
								c.name,
								c.expected,
								header,
							)
						}
					},
				)
			}
		},
	)

	t.Run(
		"span signature",
		func(t *testing.T) {
			request := http.Request{Header: getHeaders()}
			trustHandler := getHeaderSignatureTrustHandler(store)
			signature, err := trustHandler.SignSpanHeaders(
				httpbp.NewSpanHeaders(request.Header),
				time.Minute,
			)
			if err != nil {
				t.Fatalf("Got an unexpected error while trying to sign headers: %v", err)
			}

			request.Header.Add(httpbp.SpanSignatureHeader, signature)
			ctx := httpbp.PopulateBaseplateRequestContext(
				getHeaderSignatureTrustHandler(store),
			)(context.Background(), &request)

			cases := []struct {
				name     string
				key      httpbp.HeaderContextKey
				expected string
			}{
				{
					name:     "Trace ID",
					key:      httpbp.TraceIDContextKey,
					expected: traceID,
				},
				{
					name:     "Parent ID",
					key:      httpbp.ParentIDContextKey,
					expected: parentID,
				},
				{
					name:     "Span ID",
					key:      httpbp.SpanIDContextKey,
					expected: spanID,
				},
				{
					name:     "Flags",
					key:      httpbp.SpanFlagsContextKey,
					expected: flags,
				},
				{
					name:     "Sampled",
					key:      httpbp.SpanSampledContextKey,
					expected: sampled,
				},
				{
					name:     "Edge Context",
					key:      httpbp.EdgeContextContextKey,
					expected: "",
				},
			}
			for _, _c := range cases {
				c := _c
				t.Run(
					c.name,
					func(t *testing.T) {
						header := httpbp.GetHeader(ctx, c.key)
						if header != c.expected {
							t.Errorf(
								"Expected %s to be %v, got %v",
								c.name,
								c.expected,
								header,
							)
						}
					},
				)
			}
		},
	)

	t.Run(
		"both signatures",
		func(t *testing.T) {
			request := http.Request{Header: getHeaders()}
			trustHandler := getHeaderSignatureTrustHandler(store)

			signature, err := trustHandler.SignSpanHeaders(
				httpbp.NewSpanHeaders(request.Header),
				time.Minute,
			)
			if err != nil {
				t.Fatalf("Got an unexpected error while trying to sign span headers: %v", err)
			}
			request.Header.Add(httpbp.SpanSignatureHeader, signature)

			signature, err = trustHandler.SignEdgeContextHeader(
				httpbp.NewEdgeContextHeaders(request.Header),
				time.Minute,
			)
			if err != nil {
				t.Fatalf("Got an unexpected error while trying to edge context headers: %v", err)
			}

			request.Header.Add(
				httpbp.EdgeContextSignatureHeader,
				signature,
			)

			ctx := httpbp.PopulateBaseplateRequestContext(
				getHeaderSignatureTrustHandler(store),
			)(context.Background(), &request)

			cases := []struct {
				name     string
				key      httpbp.HeaderContextKey
				expected string
			}{
				{
					name:     "Trace ID",
					key:      httpbp.TraceIDContextKey,
					expected: traceID,
				},
				{
					name:     "Parent ID",
					key:      httpbp.ParentIDContextKey,
					expected: parentID,
				},
				{
					name:     "Span ID",
					key:      httpbp.SpanIDContextKey,
					expected: spanID,
				},
				{
					name:     "Flags",
					key:      httpbp.SpanFlagsContextKey,
					expected: flags,
				},
				{
					name:     "Sampled",
					key:      httpbp.SpanSampledContextKey,
					expected: sampled,
				},
				{
					name:     "Edge Context",
					key:      httpbp.EdgeContextContextKey,
					expected: edgeContext,
				},
			}
			for _, _c := range cases {
				c := _c
				t.Run(
					c.name,
					func(t *testing.T) {
						header := httpbp.GetHeader(ctx, c.key)
						if header != c.expected {
							t.Errorf(
								"Expected %s to be %v, got %v",
								c.name,
								c.expected,
								header,
							)
						}
					},
				)
			}
		},
	)
}
