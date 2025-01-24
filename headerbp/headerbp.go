package headerbp

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

const (
	// headerPrefixCanonicalHTTP and headerPrefixLower must remain the same character length
	headerPrefixCanonicalHTTP = "X-Bp-"
	headerPrefixLower         = "x-bp-"

	IsUntrustedRequestHeaderCanonicalHTTP = "X-Rddt-Untrusted"
	IsUntrustedRequestHeaderLower         = "x-rddt-untrusted"
)

// ErrNewInternalHeaderNotAllowed is returned by a client when the call tries to set an internal header is not allowlisted
var ErrNewInternalHeaderNotAllowed = fmt.Errorf("cannot send new internal headers on requests")

var setHeaderbpV2Context = func(ctx context.Context, _ map[string]string) context.Context {
	return ctx
}

// SetV2BaseplateHeadersSetter sets the function to use to set baseplate headers in the v2 library.
func SetV2BaseplateHeadersSetter(setter func(context.Context, map[string]string) context.Context) {
	setHeaderbpV2Context = setter
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

func normalizeKey(key string) string {
	if normalized, ok := normalizedKeys.Load(key); ok {
		return normalized.(string)
	}
	normalized := strings.ToLower(key)
	normalizedKeys.LoadOrStore(key, normalized)
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
	normalized := normalizeKey(key)
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
	ctx = setHeaderbpV2Context(ctx, h.headers)
	return HeadersToContext(ctx, h.headers)
}

// HeadersToContext can be used to allow interoperability with the v2 library.
func HeadersToContext(ctx context.Context, headers map[string]string) context.Context {
	return context.WithValue(ctx, headersKey{}, headers)
}

// CheckClientHeader checks if the header is allowlisted and returns an error if it is not.
func CheckClientHeader(name string, options ...CheckClientHeaderOption) error {
	cfg := &checkClientHeaders{}
	WithCheckClientHeaderOptions(options...).ApplyToCheckClientHeaders(cfg)

	if IsBaseplateHeader(name) {
		clientHeadersRejectedTotal.WithLabelValues(
			cfg.RPCType,
			cfg.Service,
			cfg.Client,
			cfg.Method,
			strings.ToLower(name),
		).Inc()
		return fmt.Errorf("%w, %q is not allowlisted", ErrNewInternalHeaderNotAllowed, name)
	}
	return nil
}

type CheckClientHeaderOption interface {
	ApplyToCheckClientHeaders(*checkClientHeaders)
}

func WithCheckClientHeaderOptions(options ...CheckClientHeaderOption) CheckClientHeaderOption {
	return &checkClientHeaders{
		commonOption: commonOption{
			applyToCheckClientHeaders: func(headers *checkClientHeaders) {
				for _, opt := range options {
					opt.ApplyToCheckClientHeaders(headers)
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

type CommonHeaderOption interface {
	CheckClientHeaderOption
	SetOutgoingHeadersOption
	NewIncomingHeadersOption
}

type commonOption struct {
	applyToCheckClientHeaders func(*checkClientHeaders)
	applyToSetOutgoingHeaders func(*setOutgoingHeaders)
	applyToNewIncomingHeaders func(*newIncomingHeaders)

	RPCType string
	Service string
	Client  string
	Method  string
}

type checkClientHeaders struct {
	commonOption
}

func (c *checkClientHeaders) ApplyToCheckClientHeaders(headers *checkClientHeaders) {
	c.applyToCheckClientHeaders(headers)
}

type newIncomingHeaders struct {
	commonOption
}

func (n *newIncomingHeaders) ApplyToNewIncomingHeaders(headers *newIncomingHeaders) {
	n.applyToNewIncomingHeaders(headers)
}

type setOutgoingHeaders struct {
	commonOption

	SetHeader func(key, value string)
}

func (s *setOutgoingHeaders) ApplyToSetOutgoingHeaders(headers *setOutgoingHeaders) {
	s.applyToSetOutgoingHeaders(headers)
}

func (c *commonOption) ApplyToCheckClientHeaders(headers *checkClientHeaders) {
	c.applyToCheckClientHeaders(headers)
}

func (c *commonOption) ApplyToSetOutgoingHeaders(headers *setOutgoingHeaders) {
	c.applyToSetOutgoingHeaders(headers)
}

func (c *commonOption) ApplyToNewIncomingHeaders(headers *newIncomingHeaders) {
	c.applyToNewIncomingHeaders(headers)
}

func WithGRPCService(service, method string) CommonHeaderOption {
	cc := commonOption{
		RPCType: "grpc",
		Service: service,
		Method:  method,
	}
	return &commonOption{
		applyToNewIncomingHeaders: func(headers *newIncomingHeaders) {
			headers.commonOption = cc
		},
	}
}

func WithGRPCClient(service, client, method string) CommonHeaderOption {
	cc := commonOption{
		RPCType: "grpc",
		Service: service,
		Client:  client,
		Method:  method,
	}
	return &commonOption{
		applyToCheckClientHeaders: func(headers *checkClientHeaders) {
			headers.commonOption = cc
		},
		applyToSetOutgoingHeaders: func(headers *setOutgoingHeaders) {
			headers.commonOption = cc
		},
	}
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
		applyToCheckClientHeaders: func(headers *checkClientHeaders) {
			headers.commonOption = cc
		},
		applyToSetOutgoingHeaders: func(headers *setOutgoingHeaders) {
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
		applyToCheckClientHeaders: func(headers *checkClientHeaders) {
			headers.commonOption = cc
		},
		applyToSetOutgoingHeaders: func(headers *setOutgoingHeaders) {
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

// SetOutgoingHeaders sets the baseplate headers in the outgoing headers if they have not already been set by the caller.
func SetOutgoingHeaders(ctx context.Context, options ...SetOutgoingHeadersOption) {
	cfg := &setOutgoingHeaders{}
	WithSetOutgoingHeadersOptions(options...).ApplyToSetOutgoingHeaders(cfg)

	headers, ok := ctx.Value(headersKey{}).(map[string]string)
	if !ok {
		return
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
}
