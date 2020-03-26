package edgecontext

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/timebp"

	"github.com/apache/thrift/lib/go/thrift"
)

// LoIDPrefix is the prefix for all LoIDs.
const LoIDPrefix = "t2_"

// ErrLoIDWrongPrefix is an error could be returned by New() when passed in LoID
// does not have the correct prefix.
var ErrLoIDWrongPrefix = errors.New("edgecontext: loid should have " + LoIDPrefix + " prefix")

// ErrNoHeader is an error could be returned by FromThriftContext() when passed
// in context does not have Edge-Request header set.
var ErrNoHeader = errors.New("edgecontext: no Edge-Request header found")

// An Impl is an initialized edge context implementation.
//
// Please call Init function to initialize it.
type Impl struct {
	store     *secrets.Store
	logger    log.Wrapper
	keysValue atomic.Value
}

var serializerPool = thrift.NewTSerializerPool(
	func() *thrift.TSerializer {
		trans := thrift.NewTMemoryBufferLen(1024)
		proto := thrift.NewTBinaryProtocolFactoryDefault().GetProtocol(trans)

		return &thrift.TSerializer{
			Transport: trans,
			Protocol:  proto,
		}
	},
)

var deserializerPool = thrift.NewTDeserializerPool(
	func() *thrift.TDeserializer {
		trans := thrift.NewTMemoryBufferLen(1024)
		proto := thrift.NewTBinaryProtocolFactoryDefault().GetProtocol(trans)

		return &thrift.TDeserializer{
			Transport: trans,
			Protocol:  proto,
		}
	},
)

type contextKey int

const (
	edgeContextKey contextKey = iota
)

// SetEdgeContext sets the given EdgeRequestContext on the context object.
func SetEdgeContext(ctx context.Context, ec *EdgeRequestContext) context.Context {
	return context.WithValue(ctx, edgeContextKey, ec)
}

// GetEdgeContext gets the current EdgeRequestContext from the context object,
// if set.
func GetEdgeContext(ctx context.Context) (ec *EdgeRequestContext, ok bool) {
	if e, success := ctx.Value(edgeContextKey).(*EdgeRequestContext); success {
		ec = e
		ok = success
	}
	return
}

// Config for Init function.
type Config struct {
	// The secret store to get the keys for jwt validation
	Store *secrets.Store
	// The logger to log key decoding errors
	Logger log.Wrapper
}

// Init intializes an Impl.
func Init(cfg Config) *Impl {
	impl := &Impl{
		store:  cfg.Store,
		logger: cfg.Logger,
	}
	if impl.logger == nil {
		impl.logger = log.NopWrapper
	}
	impl.store.AddMiddlewares(impl.validatorMiddleware)
	return impl
}

// NewArgs are the args for New function.
//
// All fields are optional.
type NewArgs struct {
	// If LoID is non-empty, it must have prefix of LoIDPrefix ("t2_").
	LoID          string
	LoIDCreatedAt time.Time

	SessionID string

	DeviceID string

	AuthToken string
}

// New creates a new EdgeRequestContext from scratch.
//
// This function should be used by services on the edge talking to clients
// directly, after talked to authentication service to get the auth token.
func New(ctx context.Context, impl *Impl, args NewArgs) (*EdgeRequestContext, error) {
	request := baseplate.NewRequest()
	if args.LoID != "" {
		if !strings.HasPrefix(args.LoID, userPrefix) {
			return nil, ErrLoIDWrongPrefix
		}
		request.Loid = &baseplate.Loid{
			ID:        args.LoID,
			CreatedMs: timebp.TimeToMilliseconds(args.LoIDCreatedAt),
		}
	}
	if args.SessionID != "" {
		request.Session = &baseplate.Session{
			ID: args.SessionID,
		}
	}
	if args.DeviceID != "" {
		request.Device = &baseplate.Device{
			ID: args.DeviceID,
		}
	}
	request.AuthenticationToken = baseplate.AuthenticationToken(args.AuthToken)

	header, err := serializerPool.WriteString(ctx, request)
	if err != nil {
		return nil, err
	}
	return &EdgeRequestContext{
		impl:   impl,
		header: header,
		raw:    args,
	}, nil
}

func fromHeader(header string, impl *Impl) (*EdgeRequestContext, error) {
	request := baseplate.NewRequest()
	if err := deserializerPool.ReadString(request, header); err != nil {
		return nil, err
	}

	raw := NewArgs{
		AuthToken: string(request.AuthenticationToken),
	}
	if request.Session != nil {
		raw.SessionID = request.Session.ID
	}
	if request.Device != nil {
		raw.DeviceID = request.Device.ID
	}
	if request.Loid != nil {
		raw.LoID = request.Loid.ID
		raw.LoIDCreatedAt = timebp.MillisecondsToTime(request.Loid.CreatedMs)
	}
	return &EdgeRequestContext{
		impl:   impl,
		header: header,
		raw:    raw,
	}, nil
}

// ContextFactory builds an *EdgeRequestContext from a context object.
type ContextFactory func(ctx context.Context, impl *Impl) (*EdgeRequestContext, error)

// FromThriftContext implements the ContextFactory interface and extracts
// EdgeRequestContext from a thrift context object.
func FromThriftContext(ctx context.Context, impl *Impl) (*EdgeRequestContext, error) {
	header, ok := thrift.GetHeader(ctx, thriftbp.HeaderEdgeRequest)
	if !ok {
		return nil, ErrNoHeader
	}

	return fromHeader(header, impl)
}

// FromHTTPContext implements the ContextFactory interface and extracts
// EdgeRequestContext from an http context object.
func FromHTTPContext(ctx context.Context, impl *Impl) (*EdgeRequestContext, error) {
	header, ok := httpbp.GetHeader(ctx, httpbp.EdgeContextContextKey)
	if !ok {
		return nil, ErrNoHeader
	}

	return fromHeader(header, impl)
}

var (
	_ ContextFactory = FromThriftContext
	_ ContextFactory = FromHTTPContext
)
