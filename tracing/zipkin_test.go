package tracing_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/timebp"
	"github.com/reddit/baseplate.go/tracing"
)

// Note that this test only tests our zipkin json encoding implementation is
// consistent (that we can decode back to the same data), it does not test that
// the implementation is correct (that it actually follows zipkin's format).
func TestZipkinSpan(t *testing.T) {
	now := time.Now().Round(time.Microsecond)
	duration := time.Millisecond * 15
	cr := now.Add(duration).Round(time.Microsecond)
	endpoint := tracing.ZipkinEndpointInfo{
		ServiceName: "my-service",
		IPv4:        "127.0.0.1",
	}
	cases := map[string]tracing.ZipkinSpan{
		"optional-absent": {
			TraceID:  1234,
			Name:     "foo",
			SpanID:   4321,
			Start:    timebp.TimestampMicrosecond(now),
			Duration: timebp.DurationMicrosecond(duration),
		},
		"optional-filled": {
			TraceID:  1234,
			Name:     "foo",
			SpanID:   4321,
			Start:    timebp.TimestampMicrosecond(now),
			Duration: timebp.DurationMicrosecond(duration),
			ParentID: 54321,
			TimeAnnotations: []tracing.ZipkinTimeAnnotation{
				{
					Endpoint:  endpoint,
					Key:       tracing.ZipkinTimeAnnotationKeyClientReceive,
					Timestamp: timebp.TimestampMicrosecond(cr),
				},
			},
			BinaryAnnotations: []tracing.ZipkinBinaryAnnotation{
				{
					Endpoint: endpoint,
					Key:      tracing.ZipkinBinaryAnnotationKeyDebug,
					Value:    true,
				},
			},
		},
	}

	for label, origin := range cases {
		t.Run(
			label,
			func(t *testing.T) {
				encoded, err := json.Marshal(origin)
				if err != nil {
					t.Fatalf("json encoding error: %v", err)
				}
				t.Logf("encoded json: %s", encoded)

				var decoded tracing.ZipkinSpan
				err = json.Unmarshal(encoded, &decoded)
				if err != nil {
					t.Fatalf("json deocding error: %v", err)
				}
				t.Logf("decoded json: %+v", decoded)
				if !reflect.DeepEqual(decoded, origin) {
					t.Errorf(
						"Decoded expected %+v, got %+v",
						origin,
						decoded,
					)
				}
			},
		)
	}
}
