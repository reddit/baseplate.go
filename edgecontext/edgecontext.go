package edgecontext

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/timebp"

	"github.com/apache/thrift/lib/go/thrift"
)

// ErrLoIDWrongPrefix is an error could be returned by New() when passed in LoID
// does not have the correct prefix.
var ErrLoIDWrongPrefix = errors.New("edgecontext: loid should have t2_ prefix")

// ErrNoHeader is an error could be returned by FromThriftContext() when passed
// in context does not have Edge-Request header set.
var ErrNoHeader = errors.New("edgecontext: no Edge-Request header found")

// global vars that will be initialized in Init function.
var (
	store *secrets.Store
)

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

// Config for Init function.
type Config struct {
	Store *secrets.Store
}

// Init the global state.
//
// All other top level functions requires Init to be called first to work,
// otherwise they might panic.
func Init(cfg Config) error {
	store = cfg.Store
	return nil
}

// NewArgs are the args for New function.
//
// All fields are optional.
type NewArgs struct {
	// If LoID is non-empty, it must have prefix of "t2_".
	LoID          string
	LoIDCreatedAt time.Time

	SessionID string

	AuthToken string
}

// New creates a new EdgeRequestContext from scratch.
//
// This function should be used by services on the edge talking to clients
// directly, after talked to authentication service to get the auth token.
func New(ctx context.Context, args NewArgs) (*EdgeRequestContext, error) {
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
	request.AuthenticationToken = baseplate.AuthenticationToken(args.AuthToken)

	header, err := serializerPool.WriteString(ctx, request)
	if err != nil {
		return nil, err
	}
	return &EdgeRequestContext{
		header: header,
		raw:    args,
	}, nil
}

// FromThriftContext extracts EdgeRequestContext from a thrift context object.
func FromThriftContext(ctx context.Context) (*EdgeRequestContext, error) {
	header, ok := thrift.GetHeader(ctx, thriftbp.HeaderEdgeRequest)
	if !ok {
		return nil, ErrNoHeader
	}

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
	if request.Loid != nil {
		raw.LoID = request.Loid.ID
		raw.LoIDCreatedAt = timebp.MillisecondsToTime(request.Loid.CreatedMs)
	}
	return &EdgeRequestContext{
		header: header,
		raw:    raw,
	}, nil
}
