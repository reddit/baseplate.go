package tracing_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/tracing"
)

type CallContainer struct {
	Calls []string
}

func (c *CallContainer) AddCall(call string, fail bool) error {
	c.Calls = append(c.Calls, call)
	if fail {
		return fmt.Errorf(call)
	}
	return nil
}

func (c *CallContainer) Reset() {
	c.Calls = nil
}

type TestCreateServerSpanHook struct {
	Calls *CallContainer
	Fail  bool
}

func (h TestCreateServerSpanHook) OnCreateServerSpan(span *tracing.Span) error {
	span.AddHooks(TestSpanHook(h))
	return h.Calls.AddCall("on-server-span-create", h.Fail)
}

type TestSpanHook struct {
	Calls *CallContainer
	Fail  bool
}

func (h TestSpanHook) OnCreateChild(parent, child *tracing.Span) error {
	return h.Calls.AddCall("on-create-child", h.Fail)
}

func (h TestSpanHook) OnPostStart(span *tracing.Span) error {
	return h.Calls.AddCall("on-start", h.Fail)
}

func (h TestSpanHook) OnPreStop(span *tracing.Span, err error) error {
	return h.Calls.AddCall("on-end", h.Fail)
}

func (h TestSpanHook) OnSetTag(span *tracing.Span, key string, value interface{}) error {
	return h.Calls.AddCall("on-set-tag", h.Fail)
}

func (h TestSpanHook) OnAddCounter(span *tracing.Span, key string, delta float64) error {
	return h.Calls.AddCall("on-add-counter", h.Fail)
}

var (
	_ tracing.CreateServerSpanHook = TestCreateServerSpanHook{}
	_ tracing.CreateChildSpanHook  = TestSpanHook{}
	_ tracing.StartStopSpanHook    = TestSpanHook{}
	_ tracing.SetSpanTagHook       = TestSpanHook{}
	_ tracing.AddSpanCounterHook   = TestSpanHook{}
)

func TestHooks(t *testing.T) {
	hook := TestCreateServerSpanHook{
		Calls: &CallContainer{},
		Fail:  false,
	}
	tracing.RegisterCreateServerSpanHooks(hook)
	defer tracing.ResetHooks()

	ctx, span := thriftbp.StartSpanFromThriftContext(context.Background(), "foo")
	span.SetTag("foo", "bar")
	opentracing.StartSpanFromContext(
		ctx,
		"bar",
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	span.AddCounter("foo", 1)
	span.Stop(ctx, nil)
	expected := []string{
		"on-server-span-create",
		"on-start",
		"on-set-tag",
		"on-create-child",
		"on-add-counter",
		"on-end",
	}
	if !reflect.DeepEqual(hook.Calls.Calls, expected) {
		t.Fatalf("Expected calls %v, got %v", expected, hook.Calls.Calls)
	}
}

func TestHookFailures(t *testing.T) {
	hook := TestCreateServerSpanHook{
		Calls: &CallContainer{},
		Fail:  true,
	}
	tracing.RegisterCreateServerSpanHooks(hook)
	defer tracing.ResetHooks()

	ctx, span := thriftbp.StartSpanFromThriftContext(context.Background(), "foo")
	span.SetTag("foo", "bar")
	opentracing.StartSpanFromContext(
		ctx,
		"bar",
		tracing.SpanTypeOption{Type: tracing.SpanTypeClient},
	)
	span.AddCounter("foo", 1.0)
	span.Stop(ctx, nil)
	expected := []string{
		"on-server-span-create",
		"on-start",
		"on-set-tag",
		"on-create-child",
		"on-add-counter",
		"on-end",
	}
	if !reflect.DeepEqual(hook.Calls.Calls, expected) {
		t.Fatalf("Expected %v:\nGot: %v", expected, hook.Calls.Calls)
	}
}
