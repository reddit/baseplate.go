package httpbp

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/signing"
)

const (
	// EdgeContextHeader is the key use to get the raw edge context from
	// the HTTP request headers.
	EdgeContextHeader = "X-Edge-Request"

	// EdgeContextSignatureHeader is the key use to get the signature for
	// the edge context headers from the HTTP request headers.
	EdgeContextSignatureHeader = "X-Edge-Request-Signature"

	// ParentIDHeader is the key use to get the span parent ID from
	// the HTTP request headers.
	ParentIDHeader = "X-Parent"

	// SpanIDHeader is the key use to get the span ID from the HTTP
	// request headers.
	SpanIDHeader = "X-Span"

	// SpanFlagsHeader is the key use to get the span flags from the HTTP
	// request headers.
	SpanFlagsHeader = "X-Flags"

	// SpanSampledHeader is the key use to get the sampled flag from the
	// HTTP request headers.
	SpanSampledHeader = "X-Sampled"

	// SpanSignatureHeader is the key use to get the signature for
	// the span headers from the HTTP request headers.
	SpanSignatureHeader = "X-Span-Signature"

	// TraceIDHeader is the key use to get the trace ID from the HTTP
	// request headers.
	TraceIDHeader = "X-Trace"
)

// Headers is an interface to collect all of the HTTP headers for a particular
// baseplate resource (spans and edge contexts) into a struct that provides an
// easy way to convert them into HTTP headers.
//
// This interface exists so we can avoid having to do runtime checks on maps to
// ensure that they have the right keys set when we are trying to sign or verify
// a set of HTTP headers.
type Headers interface {
	// AsMap returns the Headers struct as a map of header keys to header
	// values.
	AsMap() map[string]string
}

// EdgeContextHeaders implements the Headers interface for HTTP EdgeContext
// headers.
type EdgeContextHeaders struct {
	EdgeRequest string
}

// NewEdgeContextHeaders returns a new EdgeContextHeaders object from the given
// HTTP headers.
//
// The edge context header using base64 encoding for http transport, so it needs decode first
func NewEdgeContextHeaders(h http.Header) (EdgeContextHeaders, error) {
	ec, err := base64.StdEncoding.DecodeString(h.Get(EdgeContextHeader))
	return EdgeContextHeaders{EdgeRequest: string(ec)}, err
}

// SetEdgeContextHeader attach EdgeRequestContext into request/response header
//
// The base64 encoding is only for http transport
func SetEdgeContextHeader(ec *edgecontext.EdgeRequestContext, w http.ResponseWriter) {
	encoded := base64.StdEncoding.EncodeToString([]byte(ec.Header()))
	w.Header().Set(EdgeContextHeader, encoded)
}

// AsMap returns the EdgeContextHeaders as a map of header keys to header
// values.
func (s EdgeContextHeaders) AsMap() map[string]string {
	return map[string]string{
		EdgeContextHeader: s.EdgeRequest,
	}
}

// SpanHeaders implements the Headers interface for HTTP Span headers.
type SpanHeaders struct {
	TraceID  string
	ParentID string
	SpanID   string
	Flags    string
	Sampled  string
}

// NewSpanHeaders returns a new SpanHeaders object from the given HTTP headers.
func NewSpanHeaders(h http.Header) SpanHeaders {
	return SpanHeaders{
		TraceID:  h.Get(TraceIDHeader),
		ParentID: h.Get(ParentIDHeader),
		SpanID:   h.Get(SpanIDHeader),
		Flags:    h.Get(SpanFlagsHeader),
		Sampled:  h.Get(SpanSampledHeader),
	}
}

// AsMap returns the SpanHeaders as a map of header keys to header values.
func (s SpanHeaders) AsMap() map[string]string {
	return map[string]string{
		TraceIDHeader:     s.TraceID,
		ParentIDHeader:    s.ParentID,
		SpanIDHeader:      s.SpanID,
		SpanFlagsHeader:   s.Flags,
		SpanSampledHeader: s.Sampled,
	}
}

var (
	_ Headers = EdgeContextHeaders{}
	_ Headers = SpanHeaders{}
)

// HeaderTrustHandler provides an interface PopulateBaseplateRequestContext to
// verify that it should trust the HTTP headers it receives.
type HeaderTrustHandler interface {
	// TrustEdgeContext informs the function returned by PopulateBaseplateRequestContext
	// if it can trust the HTTP headers that can be used to create an edge
	// context.
	//
	// If it can trust those headers, then the headers will be copied into the
	// context object to be later used to initialize the edge context for the
	// request.
	TrustEdgeContext(r *http.Request) bool

	// TrustSpan informs the function returned by PopulateBaseplateRequestContext
	// if it can trust the HTTP headers that can be used to create a server
	// span.
	//
	// If it can trust those headers, then the headers will be copied into the
	// context object to later be used to initialize the server span for the
	// request.
	TrustSpan(r *http.Request) bool
}

// NeverTrustHeaders implements the HeaderTrustHandler interface and always
// returns false.
//
// This handler is appropriate when your service is exposed to the public
// internet and also do not expect to receive these headers anyways, or simply
// does not care to parse these headers.
type NeverTrustHeaders struct{}

// TrustEdgeContext always returns false.  The edge context headers will never
// be added to the context.
func (h NeverTrustHeaders) TrustEdgeContext(r *http.Request) bool {
	return false
}

// TrustSpan always returns false.  The span headers will never be added to the
// context.
func (h NeverTrustHeaders) TrustSpan(r *http.Request) bool {
	return false
}

// AlwaysTrustHeaders implements the HeaderTrustHandler interface and always
// returns true.
//
// This handler is appropriate when your service only accept calls from within a
// secure network and you feel comfortable always trusting these headers.
type AlwaysTrustHeaders struct{}

// TrustEdgeContext always returns true.  The edge context headers will always
// be added to the context.
func (h AlwaysTrustHeaders) TrustEdgeContext(r *http.Request) bool {
	return true
}

// TrustSpan always returns true.  The span headers will always be added to the
// context.
func (h AlwaysTrustHeaders) TrustSpan(r *http.Request) bool {
	return true
}

// TrustHeaderSignature implements the HeaderTrustHandler interface and
// checks the headers for a valid signature header.  If the headers are signed,
// then they can be trusted and the Trust request returns true.  If there is no
// signature or the signature is invalid, then the Trust request returns false.
//
// For both the span and edge context headers, the trust handler expects the
// caller to provide the signature of a message in the following format:
//
// 		"{header0}:{value0}|{header1}|{value1}|...|{headerN}:{valueN}"
//
// where the headers are sorted lexicographically.  Additionally, the signature
// should be generated using the baseplate provided `signing.Sign` function.
//
// TrustHeaderSignature provides implementations for both signing and
// verifying edge context and span headers.
//
// This handler is appropriate when your service wants to be able to trust
// headers that come from trusted sources, but also receives calls from
// un-trusted sources that you would not want to accept these headers from.  One
// example would be an HTTP API that is exposed to clients over the public
// internet where you would not trust these headers but is also used internally
// where you want to accept these headers.
type TrustHeaderSignature struct {
	secrets               *secrets.Store
	edgeContextSecretPath string
	spanSecretPath        string
}

// TrustHeaderSignatureArgs is used as input to create a new
// TrustHeaderSignature.
type TrustHeaderSignatureArgs struct {
	SecretsStore          *secrets.Store
	EdgeContextSecretPath string
	SpanSecretPath        string
}

// NewTrustHeaderSignature returns a new HMACTrustHandler that uses the
// provided TrustHeaderSignatureArgs
func NewTrustHeaderSignature(args TrustHeaderSignatureArgs) TrustHeaderSignature {
	return TrustHeaderSignature{
		secrets:               args.SecretsStore,
		edgeContextSecretPath: args.EdgeContextSecretPath,
		spanSecretPath:        args.SpanSecretPath,
	}
}

func (h TrustHeaderSignature) signHeaders(headers Headers, secretPath string, expiresIn time.Duration) (string, error) {
	secret, err := h.secrets.GetVersionedSecret(secretPath)
	if err != nil {
		return "", err
	}
	return signing.Sign(signing.SignArgs{
		Message:   headerMessage(headers),
		Key:       secret.Current,
		ExpiresIn: expiresIn,
	})
}

func (h TrustHeaderSignature) verifyHeaders(headers Headers, signature string, secretPath string) (bool, error) {
	if signature == "" {
		return false, nil
	}

	secret, err := h.secrets.GetVersionedSecret(secretPath)
	if err != nil {
		return false, err
	}

	if err = signing.Verify(headerMessage(headers), signature, secret.GetAll()...); err != nil {
		return false, err
	}
	return true, nil
}

// SignEdgeContextHeader signs the edge context header using signing.Sign.
//
// The message that is signed has the following format:
//
//		"X-Edge-Request:{headerValue}
func (h TrustHeaderSignature) SignEdgeContextHeader(headers EdgeContextHeaders, expiresIn time.Duration) (string, error) {
	return h.signHeaders(headers, h.edgeContextSecretPath, expiresIn)
}

// VerifyEdgeContextHeader verifies the edge context header using signing.Verify.
func (h TrustHeaderSignature) VerifyEdgeContextHeader(headers EdgeContextHeaders, signature string) (bool, error) {
	return h.verifyHeaders(headers, signature, h.edgeContextSecretPath)
}

// SignSpanHeaders signs the given span headers using signing.Sign.
//
// The message that is signed has the following format:
//
//		"{header0}:{value0}|{header1}|{value1}|...|{headerN}:{valueN}"
//
// where the headers are sorted lexicographically.
func (h TrustHeaderSignature) SignSpanHeaders(headers SpanHeaders, expiresIn time.Duration) (string, error) {
	return h.signHeaders(headers, h.spanSecretPath, expiresIn)
}

// VerifySpanHeaders verifies the edge context header using signing.Verify.
func (h TrustHeaderSignature) VerifySpanHeaders(headers SpanHeaders, signature string) (bool, error) {
	return h.verifyHeaders(headers, signature, h.spanSecretPath)
}

// TrustEdgeContext returns true if the request has the header
// "X-Edge-Request-Signature" set and is a valid signature of the header:
//		"X-Edge-Request"
//
// The message that should be signed is:
//
//		"X-Edge-Request:{headerValue}"
func (h TrustHeaderSignature) TrustEdgeContext(r *http.Request) bool {
	signature := r.Header.Get(EdgeContextSignatureHeader)
	edgeContextHeader, err := NewEdgeContextHeaders(r.Header)
	if err != nil {
		return false
	}
	ok, err := h.VerifyEdgeContextHeader(edgeContextHeader, signature)
	if err != nil {
		return false
	}
	return ok
}

// TrustSpan returns true if the request has the header
// "X-Span-Signature" set and is a valid signature of the headers:
//
//		"X-Flags"
//		"X-Parent"
//		"X-Sampled"
//		"X-Span"
//		"X-Trace"
//
// The message that should be signed is:
//
//		"{header0}:{value0}|{header1}|{value1}|...|{headerN}:{valueN}"
//
// where the headers are sorted lexicographically.
func (h TrustHeaderSignature) TrustSpan(r *http.Request) bool {
	signature := r.Header.Get(SpanSignatureHeader)
	ok, err := h.VerifySpanHeaders(NewSpanHeaders(r.Header), signature)
	if err != nil {
		return false
	}
	return ok
}

var (
	_ HeaderTrustHandler = AlwaysTrustHeaders{}
	_ HeaderTrustHandler = NeverTrustHeaders{}
	_ HeaderTrustHandler = TrustHeaderSignature{}
)

func headerMessage(h Headers) []byte {
	headers := h.AsMap()
	components := make([]string, 0, len(headers))
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		components = append(components, fmt.Sprintf("%s:%s", key, headers[key]))
	}
	return []byte(strings.Join(components, "|"))
}
