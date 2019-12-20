package tracing

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// ErrZeroZipkinTimestamp is the error returned when the ZipkinTimestamp field
// is zero.
var ErrZeroZipkinTimestamp = errors.New("zero zipkin timestamp")

// ZipkinSpan defines a span in zipkin's json format.
//
// It's used as an intermediate format before encoding to json string.
// It shouldn't be used directly.
//
// Reference:
// https://github.com/reddit/baseplate.py/blob/1ca8488bcd42c8786e6a3db35b2a99517fd07a99/baseplate/observers/tracing.py#L266-L280
type ZipkinSpan struct {
	// Required fields.
	TraceID  uint64          `json:"traceId"`
	Name     string          `json:"name"`
	SpanID   uint64          `json:"id"`
	Start    ZipkinTimestamp `json:"timestamp"`
	Duration ZipkinDuration  `json:"duration"`

	// parentId is actually optional,
	// but when it's absent 0 needs to be set explicitly.
	ParentID uint64 `json:"parentId"`

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
	Endpoint  ZipkinEndpointInfo `json:"endpoint"`
	Timestamp ZipkinTimestamp    `json:"timestamp"`
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
	ZipkinBinaryAnnotationKeyComponent      = "component"
	ZipkinBinaryAnnotationKeyLocalComponent = "lc"

	// Boolean values
	ZipkinBinaryAnnotationKeyDebug = "debug"
	ZipkinBinaryAnnotationKeyError = "error"
)

// ZipkinTimestamp defines zipkin's timestamp format in json.
type ZipkinTimestamp time.Time

const microsecondsPerSecond = int64(time.Second / time.Microsecond)

func (zt ZipkinTimestamp) toMicrosecond() int64 {
	t := time.Time(zt)
	ts := t.Unix() * microsecondsPerSecond
	ts += int64(t.Nanosecond()) / int64(time.Microsecond)
	return ts
}

func (zt ZipkinTimestamp) String() string {
	return time.Time(zt).String()
}

// MarshalJSON implements json.Marshaler interface, using epoch microseconds.
func (zt ZipkinTimestamp) MarshalJSON() ([]byte, error) {
	if time.Time(zt).IsZero() {
		return nil, ErrZeroZipkinTimestamp
	}
	return []byte(strconv.FormatInt(zt.toMicrosecond(), 10)), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (zt *ZipkinTimestamp) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		return nil
	}

	ts, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	// NOTE: A timestamp before the year 1678 or after 2262 would overflow this,
	// but that's OK.
	ts *= int64(time.Microsecond)
	*zt = ZipkinTimestamp(time.Unix(0, ts))
	return nil
}

// ZipkinDuration defines zipkin's time duration format in json.
type ZipkinDuration time.Duration

func (zd ZipkinDuration) String() string {
	return time.Duration(zd).String()
}

// MarshalJSON implements json.Marshaler interface, using microseconds.
func (zd ZipkinDuration) MarshalJSON() ([]byte, error) {
	n := int64(time.Duration(zd) / time.Microsecond)
	return []byte(strconv.FormatInt(n, 10)), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (zd *ZipkinDuration) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		return nil
	}

	d, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*zd = ZipkinDuration(time.Duration(d) * time.Microsecond)
	return nil
}

var (
	_ json.Marshaler   = ZipkinTimestamp{}
	_ json.Unmarshaler = (*ZipkinTimestamp)(nil)
	_ json.Marshaler   = ZipkinDuration(0)
	_ json.Unmarshaler = (*ZipkinDuration)(nil)
)
