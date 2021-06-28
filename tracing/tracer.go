package tracing

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gofrs/uuid"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/randbp"
	"github.com/reddit/baseplate.go/runtimebp"
)

var (
	_ opentracing.Tracer = (*Tracer)(nil)
)

// Configuration values for the message queue.
const (
	// Max size of serialized span in bytes.
	MaxSpanSize = 102400
	// Max number of spans allowed in the message queue at one time.
	MaxQueueSize = 10000
	// Prefix added to the queue name.
	QueueNamePrefix = "traces-"
)

func init() {
	// Register an empty Tracer implementation as opentracing's global tracer.
	opentracing.SetGlobalTracer(&globalTracer)
	// Init the global allow-list
	SetMetricsTagsAllowList(nil)
}

var globalTracer = Tracer{logger: log.NopWrapper}

// A Tracer creates and manages spans.
type Tracer struct {
	sampleRate       float64
	recorder         mqsend.MessageQueue
	logger           log.Wrapper
	endpoint         ZipkinEndpointInfo
	maxRecordTimeout time.Duration
	useUUID          bool
	uuidRemoveHyphen bool
}

// TracerConfig are the configuration values to be used in InitGlobalTracer.
type TracerConfig struct {
	// The name of the service that will be attached to every span.
	ServiceName string

	// SampleRate should be in the range of [0, 1].
	// When SampleRate >= 1, all traces will be recoreded;
	// When SampleRate <= 0, none of the traces will be recorded,
	// except the ones with debug flag set.
	//
	// Please note that SampleRate only affect top level spans created inside this
	// service. For most services the sample status will be inherited from the
	// headers from the client.
	SampleRate float64

	// Logger, if non-nil, will be used to log additional informations Record
	// returned certain errors.
	Logger log.Wrapper

	// The max timeout applied to Record function.
	//
	// If the passed in context object has an earlier deadline set,
	// that deadline will be respected instead.
	//
	// If MaxRecordTimeout <= 0,
	// Record function would run in non-blocking mode,
	// that it fails immediately if the queue is full.
	MaxRecordTimeout time.Duration

	// The name of the message queue to be used to actually send sampled spans to
	// backend service (requires Baseplate.py tracing publishing sidecar with the
	// same name configured).
	//
	// QueueName should not contain "traces-" prefix, it will be auto added.
	//
	// If QueueName is empty, no spans will be sampled,
	// including the ones with debug flag set.
	QueueName string

	// The max size of the message queue (number of messages).
	//
	// If it <=0 or > MaxQueueSize (the constant, 10000),
	// MaxQueueSize constant will be used instead.
	//
	// This is only used when QueueName is non-empty.
	MaxQueueSize int64

	// If UseUUID is set to true, when generating new trace/span IDs we will use
	// UUID4 instead of uint64.
	//
	// You should only set this to true if you know all of your upstream servers
	// can handle UUID trace ids (Baseplate.go v0.8.0+ or Baseplate.py v2.0.0+).
	//
	// By default (UUIDIntact == false), we remove the hyphens from generated uuid
	// so they are in lowercase hex format [1],
	// as some zipkin validators would reject ids with hyphens in them [2].
	// Set UUIDIntact to true to disable this behavior and keep hyphens in them.
	//
	// [1]: example: "cced093a-76ee-a418-ffdc9bb9a6453df3" -> "cced093a76eea418ffdc9bb9a6453df3"
	// [2]: https://github.com/Findorgri/zipkin/blob/ac83af336faf831b197a8af76d1b35343496d27c/zipkin/src/main/java/zipkin2/internal/HexCodec.java#L58
	UseUUID    bool
	UUIDIntact bool

	// In test code,
	// this field can be used to set the message queue the tracer publishes to,
	// usually an *mqsend.MockMessageQueue.
	//
	// This field will be ignored when QueueName is non-empty,
	// to help avoiding footgun prod code.
	//
	// DO NOT USE IN PROD CODE.
	TestOnlyMockMessageQueue mqsend.MessageQueue
}

// InitGlobalTracer initializes opentracing's global tracer.
//
// This function will try to get the first local IPv4 address of this machine
// as part of the span information send to the backend service.
// If it fails to do so, UndefinedIP will be used instead,
// and the error will be logged if logger is non-nil.
func InitGlobalTracer(cfg TracerConfig) error {
	var tracer Tracer
	if cfg.QueueName != "" {
		if cfg.MaxQueueSize <= 0 || cfg.MaxQueueSize > MaxQueueSize {
			cfg.MaxQueueSize = MaxQueueSize
		}
		recorder, err := mqsend.OpenMessageQueue(mqsend.MessageQueueConfig{
			Name:           QueueNamePrefix + cfg.QueueName,
			MaxQueueSize:   cfg.MaxQueueSize,
			MaxMessageSize: MaxSpanSize,
		})
		if err != nil {
			return err
		}
		tracer.recorder = recorder
	} else {
		tracer.recorder = cfg.TestOnlyMockMessageQueue
	}

	tracer.sampleRate = cfg.SampleRate
	tracer.useUUID = cfg.UseUUID
	tracer.uuidRemoveHyphen = !cfg.UUIDIntact

	logger := cfg.Logger
	if logger == nil {
		logger = log.NopWrapper
	}
	tracer.logger = logger

	tracer.maxRecordTimeout = cfg.MaxRecordTimeout

	ip, err := runtimebp.GetFirstIPv4()
	if err != nil {
		logger(context.Background(), `Unable to get local ip address: `+err.Error())
	}
	tracer.endpoint = ZipkinEndpointInfo{
		ServiceName: cfg.ServiceName,
		IPv4:        ip,
	}

	globalTracer = tracer
	opentracing.SetGlobalTracer(&globalTracer)
	return nil
}

type closer struct{}

func (closer) Close() error {
	return CloseTracer()
}

// InitGlobalTracerWithCloser is the combination of InitGlobalTracer and
// CloseTracer.
//
// After successful initialization,
// the returned Closer would delegate to CloseTracer upon called.
func InitGlobalTracerWithCloser(cfg TracerConfig) (io.Closer, error) {
	if err := InitGlobalTracer(cfg); err != nil {
		return nil, err
	}
	return closer{}, nil
}

// Close closes the tracer's reporting.
//
// After Close is called, no more spans will be sampled.
func (t *Tracer) Close() error {
	if t.recorder == nil {
		return nil
	}
	err := t.recorder.Close()
	t.recorder = nil
	return err
}

// Record records a span with the Recorder.
//
// Span.Stop(), Span.Finish(), and Span.FinishWithOptions() call this function
// automatically.
// In most cases that should be enough and you should not call this function
// directly.
func (t *Tracer) Record(ctx context.Context, zs ZipkinSpan) error {
	if t.recorder == nil {
		return nil
	}
	data, err := json.Marshal(zs)
	if err != nil {
		return err
	}

	if ctx.Err() != nil {
		// The request context is already canceled.
		// Use background to make sure we are still able to send out the span,
		ctx = context.Background()
	}

	timeout := t.maxRecordTimeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err = t.recorder.Send(ctx, data)
	if errors.As(err, new(mqsend.MessageTooLargeError)) {
		t.logger.Log(ctx, fmt.Sprintf(
			"Span is too big, max allowed size is %d. This can be caused by an excess amount of tags. Error: %v",
			MaxSpanSize,
			err,
		))
	}
	if errors.As(err, new(mqsend.TimedOutError)) {
		t.logger.Log(
			ctx,
			"Trace queue is full. Is trace sidecar healthy? Error: "+err.Error(),
		)
	}
	return err
}

// StartSpan implements opentracing.Tracer.
//
// For opentracing.StartSpanOptions,
// it only support the following options and will ignore all others:
//
// - ChildOfRef (in which case the parent span must be of type *Span)
//
// - StartTime
//
// - Tags
//
// It supports additional StartSpanOptions defined in this package.
//
// If the new span's type is server,
// all registered CreateServerSpanHooks will be called as well.
//
// Please note that trying to set span type via opentracing-go/ext package won't
// work, please use SpanTypeOption defined in this package instead.
func (t *Tracer) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	var sso StartSpanOptions
	for _, opt := range opts {
		sso.Apply(opt)
	}

	span := newSpan(t, operationName, sso.Type)
	if !sso.OpenTracingOptions.StartTime.IsZero() {
		span.trace.start = sso.OpenTracingOptions.StartTime
	}

	parent := findFirstParentReference(sso.OpenTracingOptions.References)
	if parent != nil {
		parent.initChildSpan(span)
	} else {
		span.trace.traceID = t.newID()
		span.trace.sampled = randbp.ShouldSampleWithRate(t.sampleRate)
		initRootSpan(span)
	}

	if span.spanType == SpanTypeServer {
		// Special handlings for server spans. See also: Span.initChildSpan.
		onCreateServerSpan(span)
		span.onStart()
	}

	for key, value := range sso.OpenTracingOptions.Tags {
		span.SetTag(key, value)
	}

	return span
}

// Inject implements opentracing.Tracer.
//
// Currently it always return opentracing.ErrInvalidCarrier as the error.
func (t *Tracer) Inject(sm opentracing.SpanContext, format interface{}, carrier interface{}) error {
	return opentracing.ErrInvalidCarrier
}

// Extract implements opentracing.Tracer.
//
// Currently it always return opentracing.ErrInvalidCarrier as the error.
func (t *Tracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	return nil, opentracing.ErrInvalidCarrier
}

func (t *Tracer) newID() string {
	if t.useUUID {
		id, err := uuid.NewV4()
		if err != nil {
			t.logger.Log(
				context.Background(),
				fmt.Sprintf("Failed to generate uuid: %v", err),
			)
			// On modern linux kernel, after the system entropy pool is initialized
			// it's guaranteed to be able to get up to 256 bytes in one call without
			// error [1], so this should never happen.
			//
			// But just in case, use fake uuid as a fallback.
			//
			// [1]: https://man7.org/linux/man-pages/man2/getrandom.2.html
			return t.fakeUUID()
		}
		return t.uuidToString(id)
	}
	return strconv.FormatUint(nonZeroRandUint64(), 10)
}

func findFirstParentReference(refs []opentracing.SpanReference) *Span {
	for _, s := range refs {
		if s.Type == opentracing.ChildOfRef {
			// Note that we only support using our type as the parent right now.
			if span, ok := s.ReferencedContext.(*Span); ok {
				return span
			}
		}
	}
	return nil
}

// CloseTracer tries to cast opentracing.GlobalTracer() into *Tracer, and calls
// its Close function.
//
// See Tracer.Close for more details.
func CloseTracer() error {
	if tracer, ok := opentracing.GlobalTracer().(*Tracer); ok {
		return tracer.Close()
	}
	return nil
}

// fakeUUID generates a fake UUID using pseudo random number generator.
//
// It does that by getting 16 bytes from PRNG,
// then feed them into uuid.FromBytes().
// It's fake as in the following senses:
//
// 1. An UUID should be generated from crypto random source, not PRNG.
//
// 2. None of the defined UUID version is just using 16 random bytes.
//
// We only use this function as a last resort,
// when we encounter errors getting enough entropy from crypto random source.
// As this still looks like an UUID so it's "good enough" for trace/span id
// purposes, and it's better than either panic or empty id.
func (t *Tracer) fakeUUID() string {
	const (
		uuidBytes   = 16
		uint32Bytes = 4
	)
	b := make([]byte, uuidBytes)
	for i := 0; i < uuidBytes; i += uint32Bytes {
		// We generate 4 random uint32 instead of using the reader to read 16 bytes
		// directly, because the reader implementation decided that even if we are
		// reading 16 bytes out of the random number generator, we only have 8 bytes
		// of randomness, because we generate a random uint64 to seed the reader.
		binary.BigEndian.PutUint32(b[i:], randbp.R.Uint32())
	}
	// It's safe to use uuid.Must here because the only way uuid.FromBytes could
	// return error is that the length of b is not 16.
	return t.uuidToString(uuid.Must(uuid.FromBytes(b)))
}

func (t *Tracer) uuidToString(id uuid.UUID) string {
	if t.uuidRemoveHyphen {
		return hex.EncodeToString(id.Bytes())
	}
	return id.String()
}
