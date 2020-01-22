package tracing

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/reddit/baseplate.go/randbp"
)

func TestDebugFlag(t *testing.T) {
	var span Span

	t.Run(
		"set",
		func(t *testing.T) {
			span.SetDebug(true)
			if span.flags != FlagMaskDebug {
				t.Errorf(
					"span.flags expected %d, got %d",
					FlagMaskDebug,
					span.flags,
				)
			}
			if !span.isDebugSet() {
				t.Errorf("span.isDebugSet() should return true")
			}
		},
	)

	t.Run(
		"set-again",
		func(t *testing.T) {
			span.SetDebug(true)
			if span.flags != FlagMaskDebug {
				t.Errorf(
					"span.flags expected %d, got %d",
					FlagMaskDebug,
					span.flags,
				)
			}
			if !span.isDebugSet() {
				t.Errorf("span.isDebugSet() should return true")
			}
		},
	)

	t.Run(
		"force-sample",
		func(t *testing.T) {
			span.sampled = false
			if !span.ShouldSample() {
				t.Error("span.ShouldSample() should return true when debug flag is set")
			}
		},
	)

	t.Run(
		"unset",
		func(t *testing.T) {
			span.SetDebug(false)
			if span.flags != 0 {
				t.Errorf(
					"span.flags expected %d, got %d",
					0,
					span.flags,
				)
			}
			if span.isDebugSet() {
				t.Errorf("span.isDebugSet() should return false")
			}
		},
	)

	t.Run(
		"unset-again",
		func(t *testing.T) {
			span.SetDebug(false)
			if span.flags != 0 {
				t.Errorf(
					"span.flags expected %d, got %d",
					0,
					span.flags,
				)
			}
			if span.isDebugSet() {
				t.Errorf("span.isDebugSet() should return false")
			}
		},
	)

	t.Run(
		"no-force-sample",
		func(t *testing.T) {
			span.sampled = false
			if span.ShouldSample() {
				t.Error("span.ShouldSample() should return false when debug flag is set")
			}
		},
	)
}

func TestDebugFlagQuick(t *testing.T) {
	f := func(flags int64) bool {
		var span Span
		span.flags = flags

		set := flags | FlagMaskDebug
		unset := set - FlagMaskDebug

		span.SetDebug(true)
		if span.flags != set {
			t.Errorf(
				"span.flags for %d after SetDebug(true) expected %d, got %d",
				flags,
				set,
				span.flags,
			)
		}
		if !span.isDebugSet() {
			t.Errorf(
				"span.isDebugSet() for %d after SetDebug(true) should be true",
				flags,
			)
		}

		span.SetDebug(false)
		if span.flags != unset {
			t.Errorf(
				"span.flags for %d after SetDebug(false) expected %d, got %d",
				flags,
				unset,
				span.flags,
			)
		}
		if span.isDebugSet() {
			t.Errorf(
				"span.isDebugSet() for %d after SetDebug(false) should be false",
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
	SpanTypeServer,
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
	ip, err := getFirstIPv4()
	if err != nil {
		t.Logf("Unable to get local ip address: %v", err)
	}
	tracer := Tracer{
		SampleRate: 0.2,
		Endpoint: ZipkinEndpointInfo{
			ServiceName: "test-service",
			IPv4:        ip,
		},
	}

	f := func(
		parentName, childName, component randomName,
		childType randomSpanType,
		flags int64,
	) bool {
		span := tracer.NewTrace(string(parentName))
		span.flags = flags
		child := span.createChild(string(childName), SpanType(childType), string(component))
		if child.parentID != span.spanID {
			t.Errorf("Parent spanID %d != child parentID %d", span.spanID, child.parentID)
		}
		if child.tracer != span.tracer {
			t.Errorf("Parent tracer %p != child tracer %p", span.tracer, child.tracer)
		}
		if child.traceID != span.traceID {
			t.Errorf("Parent traceID %d != child traceID %d", span.traceID, child.traceID)
		}
		if child.sampled != span.sampled {
			t.Errorf("Parent sampled %v != child sampled %v", span.sampled, child.sampled)
		}
		if child.flags != span.flags {
			t.Errorf("Parent flags %d != child flags %d", span.flags, child.flags)
		}
		if child.start.Equal(span.start) {
			t.Error("Child should not inherit parent's start timestamp")
		}
		if child.spanID == span.spanID {
			t.Error("Child should not inherit parent's spanID")
		}
		if child.parentID == span.parentID {
			t.Error("Child should not inherit parent's parentID")
		}
		if len(child.tags) > 1 {
			t.Error("Child should not inherit parent's tags")
		}
		if len(child.counters) > 0 {
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

func TestCreateServerSpan(t *testing.T) {
	ip, err := getFirstIPv4()
	if err != nil {
		t.Logf("Unable to get local ip address: %v", err)
	}
	tracer := Tracer{
		SampleRate: 0.2,
		Endpoint: ZipkinEndpointInfo{
			ServiceName: "test-service",
			IPv4:        ip,
		},
	}

	span := CreateServerSpan(&tracer, "foo")
	if span.spanType != SpanTypeServer {
		t.Errorf("Expected span to be a ServerSpan")
	}
	if span.start.IsZero() {
		t.Errorf("Expected span to be started")
	}
}

func TestSpanTypes(t *testing.T) {
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
			c.name,
			func(t *testing.T) {
				t.Parallel()
				span := Span{Name: "test", spanType: c.spanType}
				if span.Type().String() != c.expected {
					t.Errorf("Expected span type %s, got %s", c.expected, span.Type())
				}
			},
		)
	}
}
