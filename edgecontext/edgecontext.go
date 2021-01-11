package edgecontext

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	ecthrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/edgecontext"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/timebp"
)

// LoIDPrefix is the prefix for all LoIDs.
const LoIDPrefix = "t2_"

// ErrLoIDWrongPrefix is an error could be returned by New() when passed in LoID
// does not have the correct prefix.
var ErrLoIDWrongPrefix = errors.New("edgecontext: loid should have " + LoIDPrefix + " prefix")

// An Impl is an initialized edge context implementation.
//
// Please call Init function to initialize it.
type Impl struct {
	store     *secrets.Store
	logger    log.Wrapper
	keysValue atomic.Value
}

var (
	serializerPool   = thrift.NewTSerializerPoolSizeFactory(1024, thrift.NewTBinaryProtocolFactoryDefault())
	deserializerPool = thrift.NewTDeserializerPoolSizeFactory(1024, thrift.NewTBinaryProtocolFactoryDefault())
)

type contextKey int

const (
	edgeContextKey contextKey = iota
)

// SetEdgeContext sets the given EdgeRequestContext on the context object.
func SetEdgeContext(ctx context.Context, ec *EdgeRequestContext) context.Context {
	if ec == nil {
		return ctx
	}
	return context.WithValue(ctx, edgeContextKey, ec)
}

// GetEdgeContext gets the current EdgeRequestContext from the context object,
// if set.
func GetEdgeContext(ctx context.Context) (ec *EdgeRequestContext, ok bool) {
	ec, ok = ctx.Value(edgeContextKey).(*EdgeRequestContext)
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

	OriginServiceName string

	CountryCode string
}

// New creates a new EdgeRequestContext from scratch.
//
// This function should be used by services on the edge talking to clients
// directly, after talked to authentication service to get the auth token.
func New(ctx context.Context, impl *Impl, args NewArgs) (*EdgeRequestContext, error) {
	request := ecthrift.NewRequest()
	if args.LoID != "" {
		if !strings.HasPrefix(args.LoID, userPrefix) {
			return nil, ErrLoIDWrongPrefix
		}
		request.Loid = &ecthrift.Loid{
			ID:        args.LoID,
			CreatedMs: timebp.TimeToMilliseconds(args.LoIDCreatedAt),
		}
	}
	if args.SessionID != "" {
		request.Session = &ecthrift.Session{
			ID: args.SessionID,
		}
	}
	if args.DeviceID != "" {
		request.Device = &ecthrift.Device{
			ID: args.DeviceID,
		}
	}
	if args.OriginServiceName != "" {
		request.OriginService = &ecthrift.OriginService{
			Name: args.OriginServiceName,
		}
	}
	if args.CountryCode != "" {
		request.Geolocation = &ecthrift.Geolocation{
			CountryCode: ecthrift.CountryCode(args.CountryCode),
		}
	}
	request.AuthenticationToken = ecthrift.AuthenticationToken(args.AuthToken)

	header, err := serializerPool.WriteString(ctx, request)
	if err != nil {
		return nil, err
	}
	return &EdgeRequestContext{
		impl:   impl,
		header: header,
		raw:    args,
		ctx:    ctx,
	}, nil
}

// FromHeader returns a new EdgeRequestContext from the given header string
// using the given Impl.
func FromHeader(ctx context.Context, header string, impl *Impl) (*EdgeRequestContext, error) {
	if header == "" {
		return nil, nil
	}

	request := ecthrift.NewRequest()
	if err := deserializerPool.ReadString(ctx, request, header); err != nil {
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
	if request.OriginService != nil {
		raw.OriginServiceName = request.OriginService.Name
	}
	if request.Geolocation != nil {
		raw.CountryCode = string(request.Geolocation.CountryCode)
	}
	return &EdgeRequestContext{
		impl:   impl,
		header: header,
		raw:    raw,
		ctx:    ctx,
	}, nil
}
