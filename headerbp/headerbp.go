package headerbp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

const (
	// headerPrefixCanonicalHTTP and headerPrefixLower must remain the same character length
	headerPrefixCanonicalHTTP = "X-Bp-"
	headerPrefixLower         = "x-bp-"

	SignatureHeaderCanonicalHTTP = "X-Rddt-Headerbp-Signature"

	signatureVersion = 1
)

// ErrNewInternalHeaderNotAllowed is returned by a client when the call tries to set an internal header is not allowlisted
var ErrNewInternalHeaderNotAllowed = fmt.Errorf("cannot send new internal headers on requests")

var setV2HeadersContext = func(ctx context.Context, _ map[string]string) context.Context {
	return ctx
}

var setV2SignatureContext = func(ctx context.Context, _ string) context.Context {
	return ctx
}

// SetV2BaseplateHeadersSetter sets the function to use to set baseplate headers in the v2 library.
func SetV2BaseplateHeadersSetter(setter func(context.Context, map[string]string) context.Context) {
	setV2HeadersContext = setter
}

// SetV2BaseplateSignatureSetter sets the function to use to set baseplate signature in the v2 library.
func SetV2BaseplateSignatureSetter(setter func(context.Context, string) context.Context) {
	setV2SignatureContext = setter
}

// SetV0Setters can be used by the baseplate interop library to hook headerbp v0 into v2.
func SetV0Setters(
	setV0HeaderSetter func(func(context.Context, map[string]string) context.Context),
	setV0SignatureSetter func(func(context.Context, string) context.Context),
) {
	setV0HeaderSetter(setHeadersOnContext)
	setV0SignatureSetter(setSignatureOnContext)
}

// IsBaseplateHeader returns true if the header is for baseplate and should be propagated
func IsBaseplateHeader(key string) bool {
	if len(key) < len(headerPrefixLower) {
		return false
	}
	prefix := key[:len(headerPrefixLower)]
	return prefix == headerPrefixLower || prefix == headerPrefixCanonicalHTTP
}

type headersKey struct{} // context key storing `headers`

// cache normalized (lowercased) keys to avoid repeated allocations
var normalizedKeys sync.Map // map[string]string

func normalizeKey(key string, cacheOnMiss bool) string {
	if normalized, ok := normalizedKeys.Load(key); ok {
		return normalized.(string)
	}
	normalized := strings.ToLower(key)
	if cacheOnMiss {
		normalizedKeys.LoadOrStore(key, normalized)
	}
	return normalized
}

// IncomingHeaders is used to store baseplate headers that are received in a request to a service.
//
// An empty IncomingHeaders is unsafe to use and should be created using NewIncomingHeaders.
type IncomingHeaders struct {
	headers map[string]string

	rpcType            string
	service            string
	method             string
	estimatedSizeBytes int
}

func NewIncomingHeaders(options ...NewIncomingHeadersOption) *IncomingHeaders {
	cfg := &newIncomingHeaders{}
	WithNewIncomingHeadersOptions(options...).ApplyToNewIncomingHeaders(cfg)

	return &IncomingHeaders{
		headers: make(map[string]string),
		rpcType: cfg.RPCType,
		service: cfg.Service,
		method:  cfg.Method,
	}
}

// RecordHeader records the header to be forwarded if it is a baseplate header
func (h *IncomingHeaders) RecordHeader(key, value string) {
	if !IsBaseplateHeader(key) {
		return
	}
	normalized := normalizeKey(key, true)
	h.headers[normalized] = value
	h.estimatedSizeBytes += len(normalized) + len(value)
	serverHeadersReceivedTotal.WithLabelValues(
		h.rpcType,
		h.service,
		h.method,
		normalized,
	).Inc()
}

// SetOnContext attaches the collected baseplate headers to the context to be forwarded
func (h *IncomingHeaders) SetOnContext(ctx context.Context) context.Context {
	serverHeadersReceivedSize.WithLabelValues(
		h.rpcType,
		h.service,
		h.method,
	).Observe(float64(h.estimatedSizeBytes))
	ctx = setV2HeadersContext(ctx, h.headers)
	return setHeadersOnContext(ctx, h.headers)
}

func setHeadersOnContext(ctx context.Context, headers map[string]string) context.Context {
	return context.WithValue(ctx, headersKey{}, headers)
}

// ShouldRemoveClientHeader checks if the header is allowlisted and returns if the header should be removed
func ShouldRemoveClientHeader(name string, options ...CheckClientHeaderOption) bool {
	cfg := &shouldRemoveClientHeaders{}
	WithCheckClientHeaderOptions(options...).ApplyToShouldRemoveClientHeaders(cfg)

	if IsBaseplateHeader(name) {
		clientHeadersRejectedTotal.WithLabelValues(
			cfg.RPCType,
			cfg.Service,
			cfg.Client,
			cfg.Method,
			strings.ToLower(name),
		).Inc()
		slog.Error(
			"client header rejected",
			"header", name,
		)
		return true
	}
	return false
}

type CheckClientHeaderOption interface {
	ApplyToShouldRemoveClientHeaders(*shouldRemoveClientHeaders)
}

func WithCheckClientHeaderOptions(options ...CheckClientHeaderOption) CheckClientHeaderOption {
	return &shouldRemoveClientHeaders{
		commonOption: commonOption{
			applyToCheckClientHeaders: func(headers *shouldRemoveClientHeaders) {
				for _, opt := range options {
					opt.ApplyToShouldRemoveClientHeaders(headers)
				}
			},
		},
	}
}

type SetOutgoingHeadersOption interface {
	ApplyToSetOutgoingHeaders(*setOutgoingHeaders)
}

func WithSetOutgoingHeadersOptions(options ...SetOutgoingHeadersOption) SetOutgoingHeadersOption {
	return &setOutgoingHeaders{
		commonOption: commonOption{
			applyToSetOutgoingHeaders: func(headers *setOutgoingHeaders) {
				for _, opt := range options {
					opt.ApplyToSetOutgoingHeaders(headers)
				}
			},
		},
	}
}

type NewIncomingHeadersOption interface {
	ApplyToNewIncomingHeaders(*newIncomingHeaders)
}

func WithNewIncomingHeadersOptions(options ...NewIncomingHeadersOption) NewIncomingHeadersOption {
	opt := &newIncomingHeaders{
		commonOption: commonOption{
			applyToNewIncomingHeaders: func(headers *newIncomingHeaders) {
				for _, opt := range options {
					opt.ApplyToNewIncomingHeaders(headers)
				}
			},
		},
	}
	return opt
}

type HasSetOutgoingHeadersOption interface {
	ApplyToHasSetOutgoingHeaders(headers *hasSetOutgoingHeaders)
}

func WithHasSetOutgoingHeadersOptions(options ...HasSetOutgoingHeadersOption) HasSetOutgoingHeadersOption {
	return &hasSetOutgoingHeaders{
		commonOption: commonOption{
			applyToHasSetOutgoingHeaders: func(headers *hasSetOutgoingHeaders) {
				for _, opt := range options {
					opt.ApplyToHasSetOutgoingHeaders(headers)
				}
			},
		},
	}
}

type CommonHeaderOption interface {
	CheckClientHeaderOption
	SetOutgoingHeadersOption
	NewIncomingHeadersOption
	HasSetOutgoingHeadersOption
}

type commonOption struct {
	applyToCheckClientHeaders    func(*shouldRemoveClientHeaders)
	applyToSetOutgoingHeaders    func(*setOutgoingHeaders)
	applyToNewIncomingHeaders    func(*newIncomingHeaders)
	applyToHasSetOutgoingHeaders func(*hasSetOutgoingHeaders)

	RPCType string
	Service string
	Client  string
	Method  string
}

type shouldRemoveClientHeaders struct {
	commonOption
}

func (c *shouldRemoveClientHeaders) ApplyToShouldRemoveClientHeaders(headers *shouldRemoveClientHeaders) {
	c.applyToCheckClientHeaders(headers)
}

type newIncomingHeaders struct {
	commonOption
}

func (n *newIncomingHeaders) ApplyToNewIncomingHeaders(headers *newIncomingHeaders) {
	n.applyToNewIncomingHeaders(headers)
}

type hasSetOutgoingHeaders struct {
	commonOption
}

func (c *hasSetOutgoingHeaders) ApplyToHasSetOutgoingHeaders(headers *hasSetOutgoingHeaders) {
	c.applyToHasSetOutgoingHeaders(headers)
}

type setOutgoingHeaders struct {
	commonOption

	SetHeader func(key, value string)
}

func (s *setOutgoingHeaders) ApplyToSetOutgoingHeaders(headers *setOutgoingHeaders) {
	s.applyToSetOutgoingHeaders(headers)
}

func (c *commonOption) ApplyToShouldRemoveClientHeaders(headers *shouldRemoveClientHeaders) {
	c.applyToCheckClientHeaders(headers)
}

func (c *commonOption) ApplyToSetOutgoingHeaders(headers *setOutgoingHeaders) {
	c.applyToSetOutgoingHeaders(headers)
}

func (c *commonOption) ApplyToNewIncomingHeaders(headers *newIncomingHeaders) {
	c.applyToNewIncomingHeaders(headers)
}

func (c *commonOption) ApplyToHasSetOutgoingHeaders(headers *hasSetOutgoingHeaders) {
	c.applyToHasSetOutgoingHeaders(headers)
}

func WithHTTPService(service, method string) CommonHeaderOption {
	cc := commonOption{
		RPCType: "http",
		Service: service,
		Method:  method,
	}
	return &commonOption{
		applyToNewIncomingHeaders: func(headers *newIncomingHeaders) {
			headers.commonOption = cc
		},
	}
}

func WithHTTPClient(service, client, endpoint string) CommonHeaderOption {
	cc := commonOption{
		RPCType: "http",
		Service: service,
		Client:  client,
		Method:  endpoint,
	}
	return &commonOption{
		applyToCheckClientHeaders: func(headers *shouldRemoveClientHeaders) {
			headers.commonOption = cc
		},
		applyToSetOutgoingHeaders: func(headers *setOutgoingHeaders) {
			headers.commonOption = cc
		},
		applyToHasSetOutgoingHeaders: func(headers *hasSetOutgoingHeaders) {
			headers.commonOption = cc
		},
	}
}

func WithThriftService(service, method string) CommonHeaderOption {
	cc := commonOption{
		RPCType: "thrift",
		Service: service,
		Method:  method,
	}
	return &commonOption{
		applyToNewIncomingHeaders: func(headers *newIncomingHeaders) {
			headers.commonOption = cc
		},
	}
}

func WithThriftClient(service, client, method string) CommonHeaderOption {
	cc := commonOption{
		RPCType: "thrift",
		Service: service,
		Client:  client,
		Method:  method,
	}
	return &commonOption{
		applyToCheckClientHeaders: func(headers *shouldRemoveClientHeaders) {
			headers.commonOption = cc
		},
		applyToSetOutgoingHeaders: func(headers *setOutgoingHeaders) {
			headers.commonOption = cc
		},
		applyToHasSetOutgoingHeaders: func(headers *hasSetOutgoingHeaders) {
			headers.commonOption = cc
		},
	}
}

func WithHeaderSetter(setter func(key, value string)) SetOutgoingHeadersOption {
	return &setOutgoingHeaders{
		commonOption: commonOption{
			applyToSetOutgoingHeaders: func(headers *setOutgoingHeaders) {
				headers.SetHeader = setter
			},
		},
	}
}

type setOutgoingIdempotencyKey struct{}

func HasSetOutgoingHeaders(ctx context.Context, options ...HasSetOutgoingHeadersOption) bool {
	cfg := &hasSetOutgoingHeaders{}
	WithHasSetOutgoingHeadersOptions(options...).ApplyToHasSetOutgoingHeaders(cfg)
	hasSet, ok := ctx.Value(setOutgoingIdempotencyKey{}).(bool)
	hasSet = hasSet && ok
	if hasSet {
		clientMiddlewareIdempotencyCheckTotal.WithLabelValues(
			cfg.RPCType,
			cfg.Client,
		).Inc()
		slog.ErrorContext(
			ctx, "headerbp client middleware has beet triggered twice",
			"client", cfg.Client,
			"rpc_type", cfg.RPCType,
		)
	}
	return hasSet
}

// SetOutgoingHeaders sets the baseplate headers in the outgoing headers if they have not already been set by the caller.
func SetOutgoingHeaders(ctx context.Context, options ...SetOutgoingHeadersOption) context.Context {
	cfg := &setOutgoingHeaders{}
	WithSetOutgoingHeadersOptions(options...).ApplyToSetOutgoingHeaders(cfg)

	headers, ok := ctx.Value(headersKey{}).(map[string]string)
	if !ok {
		return ctx
	}
	var forwarded int
	var estimatedSizeBytes int
	for k, v := range headers {
		cfg.SetHeader(k, v)
		estimatedSizeBytes += len(k) + len(v)
		forwarded++
	}
	clientHeadersSentTotal.WithLabelValues(
		cfg.RPCType,
		cfg.Service,
		cfg.Client,
		cfg.Method,
	).Observe(float64(forwarded))
	clientHeadersSentSize.WithLabelValues(
		cfg.RPCType,
		cfg.Service,
		cfg.Client,
		cfg.Method,
	).Observe(float64(estimatedSizeBytes))
	return context.WithValue(ctx, setOutgoingIdempotencyKey{}, true)
}
