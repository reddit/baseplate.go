package tracing

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
	"time"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/randbp"
)

func TestDebugFlag(t *testing.T) {
	span := AsSpan(opentracing.StartSpan("test"))

	t.Run(
		"set",
		func(t *testing.T) {
			span.SetDebug(true)
			if span.trace.flags != FlagMaskDebug {
				t.Errorf(
					"span.trace.flags expected %d, got %d",
					FlagMaskDebug,
					span.trace.flags,
				)
			}
			if !span.trace.isDebugSet() {
				t.Errorf("span.trace.isDebugSet() should return true")
			}
		},
	)

	t.Run(
		"set-again",
		func(t *testing.T) {
			span.SetDebug(true)
			if span.trace.flags != FlagMaskDebug {
				t.Errorf(
					"span.trace.flags expected %d, got %d",
					FlagMaskDebug,
					span.trace.flags,
				)
			}
			if !span.trace.isDebugSet() {
				t.Errorf("span.trace.isDebugSet() should return true")
			}
		},
	)

	t.Run(
		"force-sample",
		func(t *testing.T) {
			span.trace.sampled = false
			if !span.trace.shouldSample() {
				t.Error("span.trace.shouldSample() should return true when debug flag is set")
			}
		},
	)

	t.Run(
		"unset",
		func(t *testing.T) {
			span.SetDebug(false)
			if span.trace.flags != 0 {
				t.Errorf(
					"span.trace.flags expected %d, got %d",
					0,
					span.trace.flags,
				)
			}
			if span.trace.isDebugSet() {
				t.Errorf("span.trace.isDebugSet() should return false")
			}
		},
	)

	t.Run(
		"unset-again",
		func(t *testing.T) {
			span.SetDebug(false)
			if span.trace.flags != 0 {
				t.Errorf(
					"span.trace.flags expected %d, got %d",
					0,
					span.trace.flags,
				)
			}
			if span.trace.isDebugSet() {
				t.Errorf("span.trace.isDebugSet() should return false")
			}
		},
	)

	t.Run(
		"no-force-sample",
		func(t *testing.T) {
			span.trace.sampled = false
			if span.trace.shouldSample() {
				t.Error("span.trace.shouldSample() should return false when debug flag is set")
			}
		},
	)
}

func TestDebugFlagQuick(t *testing.T) {
	f := func(flags int64) bool {
		span := AsSpan(opentracing.StartSpan("test"))

		span.trace.flags = flags

		set := flags | FlagMaskDebug
		unset := set - FlagMaskDebug

		span.SetDebug(true)
		if span.trace.flags != set {
			t.Errorf(
				"span.trace.flags for %d after SetDebug(true) expected %d, got %d",
				flags,
				set,
				span.trace.flags,
			)
		}
		if !span.trace.isDebugSet() {
			t.Errorf(
				"span.trace.isDebugSet() for %d after SetDebug(true) should be true",
				flags,
			)
		}

		span.SetDebug(false)
		if span.trace.flags != unset {
			t.Errorf(
				"span.trace.flags for %d after SetDebug(false) expected %d, got %d",
				flags,
				unset,
				span.trace.flags,
			)
		}
		if span.trace.isDebugSet() {
			t.Errorf(
				"span.trace.isDebugSet() for %d after SetDebug(false) should be false",
				flags,
			)
		}

		return !t.Failed()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

type randomSpanType SpanType

var allowedSpanTypes = []SpanType{
	SpanTypeLocal,
	SpanTypeClient,
}

func (randomSpanType) Generate(r *rand.Rand, _ int) reflect.Value {
	return reflect.ValueOf(randomSpanType(
		allowedSpanTypes[r.Intn(len(allowedSpanTypes))],
	))
}

type randomName string

const maxNameLength = 20

func (randomName) Generate(r *rand.Rand, _ int) reflect.Value {
	return reflect.ValueOf(randomName(
		randbp.GenerateRandomString(randbp.RandomStringArgs{
			R:         r,
			MaxLength: maxNameLength,
		}),
	))
}

var (
	_ quick.Generator = randomSpanType(0)
	_ quick.Generator = randomName("")
)

func TestChildSpan(t *testing.T) {
	defer func() {
		CloseTracer()
		InitGlobalTracer(TracerConfig{})
	}()
	logger, startFailing := TestWrapper(t)
	InitGlobalTracer(TracerConfig{
		SampleRate: 0.2,
		Logger:     logger,
	})
	startFailing()

	f := func(
		parentName, childName, component randomName,
		childType randomSpanType,
		flags int64,
	) bool {
		span := AsSpan(opentracing.StartSpan(string(parentName)))
		span.trace.flags = flags
		if span.getHub() == nopHub {
			t.Error("span.getHub() returned nopHub")
		}

		var child *Span
		switch SpanType(childType) {
		case SpanTypeClient:
			child = AsSpan(opentracing.StartSpan(
				string(childName),
				opentracing.ChildOf(span),
				SpanTypeOption{Type: SpanTypeClient},
			))
		case SpanTypeLocal:
			child = AsSpan(opentracing.StartSpan(
				string(childName),
				opentracing.ChildOf(span),
				LocalComponentOption{Name: string(component)},
			))
		}

		if child.trace.parentID != span.trace.spanID {
			t.Errorf("Parent spanID %q != child parentID %q", span.trace.spanID, child.trace.parentID)
		}
		if child.trace.tracer != span.trace.tracer {
			t.Errorf(
				"Parent tracer %p(%#v) != child tracer %p(%#v)",
				span.trace.tracer, span.trace.tracer,
				child.trace.tracer, child.trace.tracer,
			)
		}
		if child.trace.traceID != span.trace.traceID {
			t.Errorf("Parent traceID %q != child traceID %q", span.trace.traceID, child.trace.traceID)
		}
		if child.trace.sampled != span.trace.sampled {
			t.Errorf("Parent sampled %v != child sampled %v", span.trace.sampled, child.trace.sampled)
		}
		if child.trace.flags != span.trace.flags {
			t.Errorf("Parent flags %d != child flags %d", span.trace.flags, child.trace.flags)
		}
		if child.trace.start.Equal(span.trace.start) {
			t.Error("Child should not inherit parent's start timestamp")
		}
		if child.trace.spanID == span.trace.spanID {
			t.Error("Child should not inherit parent's spanID")
		}
		if child.trace.parentID == span.trace.parentID {
			t.Error("Child should not inherit parent's parentID")
		}
		if len(child.trace.tags) > 1 {
			t.Error("Child should not inherit parent's tags")
		}
		if len(child.trace.counters) > 0 {
			t.Error("Child should not inherit parent's counters")
		}
		if child.getHub() != span.getHub() {
			t.Errorf("Parent hub %#v != child hub %#v", span.getHub(), child.getHub())
		}
		if t.Failed() {
			t.Logf("parent: %+v, child: %+v", span, child)
		}
		return !t.Failed()
	}

	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestSpanTypeStrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		spanType SpanType
		expected string
	}{
		{
			name:     server,
			spanType: SpanTypeServer,
			expected: server,
		},
		{
			name:     local,
			spanType: SpanTypeLocal,
			expected: local,
		},
		{
			name:     client,
			spanType: SpanTypeClient,
			expected: client,
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.expected,
			func(t *testing.T) {
				t.Parallel()
				if c.spanType.String() != c.expected {
					t.Errorf(
						"Expected SpanType.String to be %s, got %v",
						c.expected,
						c.spanType,
					)
				}
			},
		)
	}
}

func TestHeadersAnySet(t *testing.T) {
	t.Parallel()

	sampled := true
	notSampled := false

	const (
		traceID       = "1"
		spanID        = "2"
		flags         = "0"
		invalidIntVal = "invalid"
	)
	invalidID := strings.Repeat("1", maxIDLength+1)

	cases := []struct {
		name     string
		headers  Headers
		expected bool
	}{
		// Typical cases, generally you will either have all the headers set or
		// none of them.
		{
			name:     "none-set",
			headers:  Headers{},
			expected: false,
		},
		{
			name: "all-set",
			headers: Headers{
				TraceID: traceID,
				SpanID:  spanID,
				Flags:   flags,
				Sampled: &sampled,
			},
			expected: true,
		},
		{
			name: "all-set-sampled-false",
			headers: Headers{
				TraceID: traceID,
				SpanID:  spanID,
				Flags:   flags,
				Sampled: &notSampled,
			},
			expected: true,
		},

		// Test only a single value set, but set to a value you would expect.
		{
			name: "trace-id-set",
			headers: Headers{
				TraceID: traceID,
			},
			expected: true,
		},
		{
			name: "span-id-set",
			headers: Headers{
				SpanID: spanID,
			},
			expected: true,
		},
		{
			name: "flags-set",
			headers: Headers{
				Flags: flags,
			},
			expected: true,
		},
		{
			name: "sampled-set",
			headers: Headers{
				Sampled: &sampled,
			},
			expected: true,
		},

		// Test having values set, but to something that is either invalid or could
		// be trickier.  Headers.AnySet should only care that something is set, not
		// what it is set to or if it is valid.
		{
			name: "trace-id-set",
			headers: Headers{
				TraceID: invalidID,
			},
			expected: true,
		},
		{
			name: "span-id-set",
			headers: Headers{
				SpanID: invalidID,
			},
			expected: true,
		},
		{
			name: "flags-set",
			headers: Headers{
				Flags: invalidIntVal,
			},
			expected: true,
		},
		{
			name: "sampled-set-but-false",
			headers: Headers{
				Sampled: &notSampled,
			},
			expected: true,
		},
		{
			name: "all-set-invalid",
			headers: Headers{
				TraceID: invalidID,
				SpanID:  invalidID,
				Flags:   invalidID,
				Sampled: &notSampled,
			},
			expected: true,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				if c.headers.AnySet() != c.expected {
					t.Fatalf("AnySet did not match, expected %v, got %v", c.expected, c.headers.AnySet())
				}
			},
		)
	}
}

func TestHeadersParseTraceID(t *testing.T) {
	t.Parallel()

	type expectation struct {
		id string
		ok bool
	}
	cases := []struct {
		name     string
		headers  Headers
		expected expectation
	}{
		{
			name:    "uint64",
			headers: Headers{TraceID: "1"},
			expected: expectation{
				id: "1",
				ok: true,
			},
		},
		{
			name:    "uuid",
			headers: Headers{TraceID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8"},
			expected: expectation{
				id: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
				ok: true,
			},
		},
		{
			name:    "invalid",
			headers: Headers{TraceID: strings.Repeat("1", maxIDLength+1)},
			expected: expectation{
				ok: false,
			},
		},
		{
			name:    "not-set",
			headers: Headers{},
			expected: expectation{
				ok: false,
			},
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				id, ok := c.headers.ParseTraceID()
				if ok != c.expected.ok {
					t.Errorf("ok mismatch, expected %v, got %v", c.expected.ok, ok)
				}
				// Don't test the value for "id" if ok is false because it should not
				// be relied on.
				if !ok {
					return
				}

				if id != c.expected.id {
					t.Errorf("parsed ID mismatch, expected %q, got %q", c.expected.id, id)
				}
			},
		)
	}
}

func TestHeadersParseSpanID(t *testing.T) {
	t.Parallel()

	type expectation struct {
		id string
		ok bool
	}
	cases := []struct {
		name     string
		headers  Headers
		expected expectation
	}{
		{
			name:    "uint64",
			headers: Headers{SpanID: "1"},
			expected: expectation{
				id: "1",
				ok: true,
			},
		},
		{
			name:    "uuid",
			headers: Headers{SpanID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8"},
			expected: expectation{
				id: "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
				ok: true,
			},
		},
		{
			name:    "invalid",
			headers: Headers{TraceID: strings.Repeat("1", maxIDLength+1)},
			expected: expectation{
				ok: false,
			},
		},
		{
			name:    "not-set",
			headers: Headers{},
			expected: expectation{
				ok: false,
			},
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				id, ok := c.headers.ParseSpanID()
				if ok != c.expected.ok {
					t.Errorf("ok mismatch, expected %v, got %v", c.expected.ok, ok)
				}
				// Don't test the value for "id" if ok is false because it should not
				// be relied on.
				if !ok {
					return
				}

				if id != c.expected.id {
					t.Errorf("parsed ID mismatch, expected %q, got %q", c.expected.id, id)
				}
			},
		)
	}
}

func TestHeadersParseFlags(t *testing.T) {
	t.Parallel()

	type expectation struct {
		id int64
		ok bool
	}
	cases := []struct {
		name     string
		headers  Headers
		expected expectation
	}{
		{
			name:    "ok",
			headers: Headers{Flags: "1"},
			expected: expectation{
				id: 1,
				ok: true,
			},
		},
		{
			name:    "invalid",
			headers: Headers{Flags: "foo"},
			expected: expectation{
				ok: false,
			},
		},
		{
			name:    "not-set",
			headers: Headers{},
			expected: expectation{
				ok: false,
			},
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				id, ok := c.headers.ParseFlags()
				if ok != c.expected.ok {
					t.Errorf("ok mismatch, expected %v, got %v", c.expected.ok, ok)
				}
				// Don't test the value for "flags" if ok is false because it should not
				// be relied on.
				if !ok {
					return
				}

				if id != c.expected.id {
					t.Errorf("parsed ID mismatch, expected %d, got %d", c.expected.id, id)
				}
			},
		)
	}
}

func TestHeadersParseSampled(t *testing.T) {
	t.Parallel()

	sampled := true
	notSampled := false

	type expectation struct {
		sampled bool
		ok      bool
	}
	cases := []struct {
		name     string
		headers  Headers
		expected expectation
	}{
		{
			name:    "true",
			headers: Headers{Sampled: &sampled},
			expected: expectation{
				sampled: true,
				ok:      true,
			},
		},
		{
			name:    "false",
			headers: Headers{Sampled: &notSampled},
			expected: expectation{
				sampled: false,
				ok:      true,
			},
		},
		{
			name:    "not-set",
			headers: Headers{},
			expected: expectation{
				ok: false,
			},
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				sampled, ok := c.headers.ParseSampled()
				if ok != c.expected.ok {
					t.Errorf("ok mismatch, expected %v, got %v", c.expected.ok, ok)
				}
				// Don't test the value for "sampled" if ok is false because it should not
				// be relied on.
				if !ok {
					return
				}

				if sampled != c.expected.sampled {
					t.Errorf(
						"parsed sampled mismatch, expected %v, got %v",
						c.expected.sampled,
						sampled,
					)
				}
			},
		)
	}
}

func TestStartAndFinishTimes(t *testing.T) {
	startTime := time.Unix(1, 0)
	stopTime := startTime.Add(time.Second)
	span := AsSpan(opentracing.StartSpan(
		"test",
		opentracing.StartTime(startTime),
	))
	if !span.StartTime().Equal(startTime) {
		t.Fatalf("start time mismatch, expected %v, got %v", startTime, span.StartTime())
	}
	if !span.StopTime().IsZero() {
		t.Fatalf("stop time should be zero, got %v", span.StopTime())
	}

	span.FinishWithOptions(opentracing.FinishOptions{FinishTime: stopTime})
	if !span.StopTime().Equal(stopTime) {
		t.Fatalf("start time mismatch, expected %v, got %v", stopTime, span.StopTime())
	}
}
