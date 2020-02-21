package tracing

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

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
		randbp.GenerateRandomString(r, maxNameLength, []rune(randbp.Base64Runes)),
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
			t.Errorf("Parent spanID %d != child parentID %d", span.trace.spanID, child.trace.parentID)
		}
		if child.trace.tracer != span.trace.tracer {
			t.Errorf(
				"Parent tracer %p(%#v) != child tracer %p(%#v)",
				span.trace.tracer, span.trace.tracer,
				child.trace.tracer, child.trace.tracer,
			)
		}
		if child.trace.traceID != span.trace.traceID {
			t.Errorf("Parent traceID %d != child traceID %d", span.trace.traceID, child.trace.traceID)
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
