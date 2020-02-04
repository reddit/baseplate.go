package httpbp_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/reddit/baseplate.go/httpbp"
)

const (
	traceID     = "1234"
	parentID    = "5678"
	spanID      = "90123"
	flags       = "0"
	sampled     = "1"
	edgeContext = "edge-context"
)

func getHeaders() http.Header {
	headers := make(http.Header)
	for k, v := range map[string]string{
		httpbp.TraceIDHeader:     traceID,
		httpbp.ParentIDHeader:    parentID,
		httpbp.SpanIDHeader:      spanID,
		httpbp.SpanFlagsHeader:   flags,
		httpbp.SpanSampledHeader: sampled,
		httpbp.EdgeContextHeader: edgeContext,
	} {
		headers.Add(k, v)
	}
	return headers
}

func TestSetAndGetHeaders(t *testing.T) {
	t.Parallel()

	request := http.Request{Header: getHeaders()}
	ctx := httpbp.PopulateRequestContext(
		httpbp.AlwaysTrustHeaders{},
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
				t.Parallel()
				header, _ := httpbp.GetHeader(ctx, c.key)
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

func TestNewSpanHeaders(t *testing.T) {
	t.Parallel()

	headers := getHeaders()
	spanHeaders := httpbp.NewSpanHeaders(headers)

	cases := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "Trace ID",
			value:    spanHeaders.TraceID,
			expected: traceID,
		},
		{
			name:     "Parent ID",
			value:    spanHeaders.ParentID,
			expected: parentID,
		},
		{
			name:     "Span ID",
			value:    spanHeaders.SpanID,
			expected: spanID,
		},
		{
			name:     "Flags",
			value:    spanHeaders.Flags,
			expected: flags,
		},
		{
			name:     "Sampled",
			value:    spanHeaders.Sampled,
			expected: sampled,
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()
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
	}
}

func TestSpanHeadersAsMap(t *testing.T) {
	t.Parallel()

	headers := getHeaders()
	spanHeaders := httpbp.NewSpanHeaders(headers)

	cases := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "Trace ID",
			key:      httpbp.TraceIDHeader,
			expected: traceID,
		},
		{
			name:     "Parent ID",
			key:      httpbp.ParentIDHeader,
			expected: parentID,
		},
		{
			name:     "Span ID",
			key:      httpbp.SpanIDHeader,
			expected: spanID,
		},
		{
			name:     "Flags",
			key:      httpbp.SpanFlagsHeader,
			expected: flags,
		},
		{
			name:     "Sampled",
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
	}
}

func TestNewEdgeContextHeaders(t *testing.T) {
	t.Parallel()

	headers := getHeaders()
	edgeContextHeaders := httpbp.NewEdgeContextHeaders(headers)

	cases := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "Edge Context",
			value:    edgeContextHeaders.EdgeRequest,
			expected: edgeContext,
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()
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
	}
}

func TestEdgeContextHeadersAsMap(t *testing.T) {
	t.Parallel()

	headers := getHeaders()
	edgeContextHeaders := httpbp.NewEdgeContextHeaders(headers)

	asHTTPHeadersCases := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "Edge Context",
			key:      httpbp.EdgeContextHeader,
			expected: edgeContext,
		},
	}
	for _, _c := range asHTTPHeadersCases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()
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
	}
}
