package tracing

import (
	"github.com/reddit/baseplate.go/timebp"
)

// ZipkinSpan defines a span in zipkin's json format.
//
// It's used as an intermediate format before encoding to json string.
// It shouldn't be used directly.
//
// Reference:
// https://github.com/reddit/baseplate.py/blob/1ca8488bcd42c8786e6a3db35b2a99517fd07a99/baseplate/observers/tracing.py#L266-L280
type ZipkinSpan struct {
	// Required fields.
	TraceID  string                      `json:"traceId"`
	Name     string                      `json:"name"`
	SpanID   string                      `json:"id"`
	Start    timebp.TimestampMicrosecond `json:"timestamp"`
	Duration timebp.DurationMicrosecond  `json:"duration"`

	// parentId is optional,
	ParentID string `json:"parentId,omitempty"`

	// Annotations are all optional.
	TimeAnnotations   []ZipkinTimeAnnotation   `json:"annotations,omitempty"`
	BinaryAnnotations []ZipkinBinaryAnnotation `json:"binaryAnnotations,omitempty"`
}

// ZipkinEndpointInfo defines Zipkin's endpoint json format.
type ZipkinEndpointInfo struct {
	ServiceName string `json:"serviceName"`
	IPv4        string `json:"ipv4"`
}

// ZipkinTimeAnnotation defines Zipkin's time annotation json format.
type ZipkinTimeAnnotation struct {
	Endpoint  ZipkinEndpointInfo          `json:"endpoint"`
	Timestamp timebp.TimestampMicrosecond `json:"timestamp"`
	// In time annotations the value is actually the timestamp and the key is
	// actually the value.
	Key string `json:"value"`
}

// ZipkinBinaryAnnotation defines Zipkin's binary annotation json format.
type ZipkinBinaryAnnotation struct {
	Endpoint ZipkinEndpointInfo `json:"endpoint"`
	Key      string             `json:"key"`
	Value    interface{}        `json:"value"`
}

// Zipkin span well-known time annotation keys.
const (
	ZipkinTimeAnnotationKeyClientReceive = "cr"
	ZipkinTimeAnnotationKeyClientSend    = "cs"
	ZipkinTimeAnnotationKeyServerReceive = "sr"
	ZipkinTimeAnnotationKeyServerSend    = "ss"
)

// Zipkin span well-known binary annotation keys.
const (
	// String values
	ZipkinBinaryAnnotationKeyComponent = "component"

	// Boolean values
	ZipkinBinaryAnnotationKeyDebug   = "debug"
	ZipkinBinaryAnnotationKeyError   = "error"
	ZipkinBinaryAnnotationKeyTimeOut = "timed_out"
)
