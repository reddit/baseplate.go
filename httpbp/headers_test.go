package httpbp_test

import (
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/secrets"
)

const (
	traceID     = "1234"
	parentID    = "5678"
	spanID      = "90123"
	flags       = "0"
	sampled     = "1"
	edgeContext = "edge-context!?$*&()'-=@~"
)

var b64EdgeContext = base64.StdEncoding.EncodeToString([]byte(edgeContext))

func getHeaders() http.Header {
	headers := make(http.Header)
	for k, v := range map[string]string{
		httpbp.TraceIDHeader:     traceID,
		httpbp.ParentIDHeader:    parentID,
		httpbp.SpanIDHeader:      spanID,
		httpbp.SpanFlagsHeader:   flags,
		httpbp.SpanSampledHeader: sampled,
		httpbp.EdgeContextHeader: b64EdgeContext,
	} {
		headers.Add(k, v)
	}
	return headers
}

func getTrustHeaderSignature(secretsStore *secrets.Store) httpbp.TrustHeaderSignature {
	return httpbp.NewTrustHeaderSignature(httpbp.TrustHeaderSignatureArgs{
		SecretsStore:          secretsStore,
		EdgeContextSecretPath: "secret/http/edge-context-signature",
		SpanSecretPath:        "secret/http/span-signature",
	})
}

func TestSpanHeaders(t *testing.T) {
	t.Parallel()

	headers := getHeaders()
	spanHeaders := httpbp.NewSpanHeaders(headers)

	cases := []struct {
		name     string
		value    string
		key      string
		expected string
	}{
		{
			name:     "Trace ID",
			value:    spanHeaders.TraceID,
			key:      httpbp.TraceIDHeader,
			expected: traceID,
		},
		{
			name:     "Parent ID",
			value:    spanHeaders.ParentID,
			key:      httpbp.ParentIDHeader,
			expected: parentID,
		},
		{
			name:     "Span ID",
			value:    spanHeaders.SpanID,
			key:      httpbp.SpanIDHeader,
			expected: spanID,
		},
		{
			name:     "Flags",
			value:    spanHeaders.Flags,
			key:      httpbp.SpanFlagsHeader,
			expected: flags,
		},
		{
			name:     "Sampled",
			value:    spanHeaders.Sampled,
			key:      httpbp.SpanSampledHeader,
			expected: sampled,
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()

				t.Run(
					"value",
					func(t *testing.T) {
						if c.value != c.expected {
							t.Errorf(
								"Expected %s to be %v, got %v",
								c.name,
								c.expected,
								c.value,
							)
						}
					},
				)

				t.Run(
					"AsMap",
					func(t *testing.T) {
						got := spanHeaders.AsMap()[c.key]
						if got != c.expected {
							t.Errorf(
								"Expected %s to be %v, got %v",
								c.name,
								c.expected,
								got,
							)
						}
					},
				)
			},
		)
	}
}

func TestNewEdgeContextHeaders(t *testing.T) {
	t.Parallel()

	headers := getHeaders()
	edgeContextHeaders, err := httpbp.NewEdgeContextHeaders(headers)
	if err != nil {
		t.Fatalf("Got an unexpected error while getting new edge context header: %v", err)
	}

	cases := []struct {
		name     string
		value    string
		key      string
		expected string
	}{
		{
			name:     "Edge Context",
			value:    edgeContextHeaders.EdgeRequest,
			key:      httpbp.EdgeContextHeader,
			expected: edgeContext,
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()

				t.Run(
					"value",
					func(t *testing.T) {
						if c.value != c.expected {
							t.Errorf(
								"Expected %s to be %v, got %v",
								c.name,
								c.expected,
								c.value,
							)
						}
					},
				)

				t.Run(
					"AsMap",
					func(t *testing.T) {
						got := edgeContextHeaders.AsMap()[c.key]
						if got != c.expected {
							t.Errorf(
								"Expected %s to be %v, got %v",
								c.name,
								c.expected,
								got,
							)
						}
					},
				)
			},
		)
	}
}

func TestAlwaysTrustHeaders(t *testing.T) {
	t.Parallel()

	request := &http.Request{Header: getHeaders()}
	truster := httpbp.AlwaysTrustHeaders{}

	if !truster.TrustSpan(request) {
		t.Errorf("did not trust span headers")
	}

	if !truster.TrustEdgeContext(request) {
		t.Errorf("did not trust edge context headers")
	}
}

func TestNeverTrustHeaders(t *testing.T) {
	t.Parallel()

	request := &http.Request{Header: getHeaders()}
	truster := httpbp.NeverTrustHeaders{}

	if truster.TrustSpan(request) {
		t.Errorf("trusted span headers")
	}

	if truster.TrustEdgeContext(request) {
		t.Errorf("trusted edge context headers")
	}
}

func TestTrustHeaderSignatureSignAndVerify(t *testing.T) {
	t.Parallel()

	store := newSecretsStore(t)
	defer store.Close()

	t.Run(
		"edge context",
		func(t *testing.T) {
			request := http.Request{Header: getHeaders()}
			trustHandler := getTrustHeaderSignature(store)
			ech, _ := httpbp.NewEdgeContextHeaders(request.Header)
			signature, err := trustHandler.SignEdgeContextHeader(
				ech,
				time.Minute,
			)
			if err != nil {
				t.Fatalf("Got an unexpected error while trying to sign headers: %v", err)
			}
			ech, err = httpbp.NewEdgeContextHeaders(request.Header)
			if err != nil {
				t.Errorf("Got an unexpected error while decoding the edge context: %w", err)
			}
			ok, err := trustHandler.VerifyEdgeContextHeader(
				ech,
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
			trustHandler := getTrustHeaderSignature(store)
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

func TestInvalidEdgeContextHeader(t *testing.T) {
	store := newSecretsStore(t)
	defer store.Close()

	invalidEdgeContextHeader := &http.Request{Header: getHeaders()}
	invalidEdgeContextHeader.Header.Set(httpbp.EdgeContextHeader, "==")

	truster := getTrustHeaderSignature(store)
	_, err := httpbp.NewEdgeContextHeaders(invalidEdgeContextHeader.Header)
	if err == nil {
		t.Fatal("Not raising expected decoding error")
	}
	ecSignature, err := truster.SignEdgeContextHeader(
		httpbp.EdgeContextHeaders{EdgeRequest: "=="},
		time.Minute,
	)
	if err != nil {
		t.Fatalf("SignEdgeContextHeader returned error: %v", err)
	}
	invalidEdgeContextHeader.Header.Set(httpbp.EdgeContextSignatureHeader, ecSignature)

	if truster.TrustEdgeContext(invalidEdgeContextHeader) {
		t.Error("trust mismatch, expected False, got True")
	}
}

func TestTrustHeaderSignature(t *testing.T) {
	t.Parallel()

	store := newSecretsStore(t)
	defer store.Close()

	baseRequest := &http.Request{Header: getHeaders()}

	truster := getTrustHeaderSignature(store)
	ech, err := httpbp.NewEdgeContextHeaders(baseRequest.Header)
	if err != nil {
		t.Fatalf("Got an unexpected error while getting new edge context header: %v", err)
	}
	ecSignature, err := truster.SignEdgeContextHeader(
		ech,
		time.Minute,
	)
	if err != nil {
		t.Fatalf("Got an unexpected error while trying to sign edge context headers: %v", err)
	}
	spanSignature, err := truster.SignSpanHeaders(
		httpbp.NewSpanHeaders(baseRequest.Header),
		time.Minute,
	)
	if err != nil {
		t.Fatalf("Got an unexpected error while trying to sign span headers: %v", err)
	}

	bothSignatures := &http.Request{Header: getHeaders()}
	bothSignatures.Header.Set(httpbp.EdgeContextSignatureHeader, ecSignature)
	bothSignatures.Header.Set(httpbp.SpanSignatureHeader, spanSignature)

	ecOnly := &http.Request{Header: getHeaders()}
	ecOnly.Header.Set(httpbp.EdgeContextSignatureHeader, ecSignature)

	spanOnly := &http.Request{Header: getHeaders()}
	spanOnly.Header.Set(httpbp.SpanSignatureHeader, spanSignature)

	type expectation struct {
		edgeContext bool
		span        bool
	}
	cases := []struct {
		name     string
		request  *http.Request
		expected expectation
	}{
		{
			name:    "no-signature",
			request: baseRequest,
			expected: expectation{
				edgeContext: false,
				span:        false,
			},
		},
		{
			name:    "span-only",
			request: spanOnly,
			expected: expectation{
				edgeContext: false,
				span:        true,
			},
		},
		{
			name:    "edgecontext-only",
			request: ecOnly,
			expected: expectation{
				edgeContext: true,
				span:        false,
			},
		},
		{
			name:    "both-signatures",
			request: bothSignatures,
			expected: expectation{
				edgeContext: true,
				span:        true,
			},
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()

				t.Run(
					"TrustSpan",
					func(t *testing.T) {
						if truster.TrustSpan(c.request) != c.expected.span {
							t.Errorf(
								"trust mismatch, expected %v, got %v",
								c.expected.span,
								truster.TrustSpan(c.request),
							)
						}
					},
				)

				t.Run(
					"TrustEdgeContext",
					func(t *testing.T) {
						if truster.TrustEdgeContext(c.request) != c.expected.edgeContext {
							t.Errorf(
								"trust mismatch, expected %v, got %v",
								c.expected.edgeContext,
								truster.TrustEdgeContext(c.request),
							)
						}
					},
				)
			},
		)
	}
}
