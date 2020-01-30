package httpbp

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/signing"
)

const (
	// EdgeContextSignatureHeader is the key use to get the signature for
	// the edge context headers from the HTTP request headers.
	EdgeContextSignatureHeader = "X-Edge-Request-Signature"

	// SpanSignatureHeader is the key use to get the signature for
	// the span headers from the HTTP request headers.
	SpanSignatureHeader = "X-Span-Signature"
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

// HeaderSignatureTrustHandler implements the HeaderTrustHandler interface and
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
// HeaderSignatureTrustHandler provides implementations for both signing and
// verifying edge context and span headers.
type HeaderSignatureTrustHandler struct {
	secrets               *secrets.Store
	edgeContextSecretPath string
	spanSecretPath        string
}

// HeaderSignatureTrustHandlerArgs is used as input to create a new
// HeaderSignatureTrustHandler.
type HeaderSignatureTrustHandlerArgs struct {
	SecretsStore          *secrets.Store
	EdgeContextSecretPath string
	SpanSecretPath        string
}

// NewHeaderSignatureTrustHandler returns a new HMACTrustHandler that uses the
// provided HeaderSignatureTrustHandlerArgs
func NewHeaderSignatureTrustHandler(args HeaderSignatureTrustHandlerArgs) HeaderSignatureTrustHandler {
	return HeaderSignatureTrustHandler{
		secrets:               args.SecretsStore,
		edgeContextSecretPath: args.EdgeContextSecretPath,
		spanSecretPath:        args.SpanSecretPath,
	}
}

func (h HeaderSignatureTrustHandler) signHeaders(headers Headers, secretPath string, expiresIn time.Duration) (string, error) {
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

func (h HeaderSignatureTrustHandler) verifyHeaders(headers Headers, signature string, secretPath string) (bool, error) {
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
func (h HeaderSignatureTrustHandler) SignEdgeContextHeader(headers *EdgeContextHeaders, expiresIn time.Duration) (string, error) {
	return h.signHeaders(headers, h.edgeContextSecretPath, expiresIn)
}

// VerifyEdgeContextHeader verifies the edge context header using signing.Verify.
func (h HeaderSignatureTrustHandler) VerifyEdgeContextHeader(headers *EdgeContextHeaders, signature string) (bool, error) {
	return h.verifyHeaders(headers, signature, h.edgeContextSecretPath)
}

// SignSpanHeaders signs the given span headers using signing.Sign.
//
// The message that is signed has the following format:
//
//		"{header0}:{value0}|{header1}|{value1}|...|{headerN}:{valueN}"
//
// where the headers are sorted lexicographically.
func (h HeaderSignatureTrustHandler) SignSpanHeaders(headers *SpanHeaders, expiresIn time.Duration) (string, error) {
	return h.signHeaders(headers, h.spanSecretPath, expiresIn)
}

// VerifySpanHeaders verifies the edge context header using signing.Verify.
func (h HeaderSignatureTrustHandler) VerifySpanHeaders(headers *SpanHeaders, signature string) (bool, error) {
	return h.verifyHeaders(headers, signature, h.spanSecretPath)
}

// TrustEdgeContext returns true if the request has the header
// "X-Edge-Request-Signature" set and is a valid signature of the header:
//		"X-Edge-Request"
//
// The message that should be signed is:
//
//		"X-Edge-Request:{headerValue}"
func (h HeaderSignatureTrustHandler) TrustEdgeContext(r *http.Request) bool {
	signature := r.Header.Get(EdgeContextSignatureHeader)
	ok, err := h.VerifyEdgeContextHeader(NewEdgeContextHeaders(r.Header), signature)
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
func (h HeaderSignatureTrustHandler) TrustSpan(r *http.Request) bool {
	signature := r.Header.Get(SpanSignatureHeader)
	ok, err := h.VerifySpanHeaders(NewSpanHeaders(r.Header), signature)
	if err != nil {
		return false
	}
	return ok
}

var (
	_ HeaderTrustHandler = AlwaysTrustHeaders{}
	_ HeaderTrustHandler = HeaderSignatureTrustHandler{}
	_ HeaderTrustHandler = NeverTrustHeaders{}
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
