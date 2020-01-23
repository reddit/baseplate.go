package tracing_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

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

type TestBaseplateHook struct {
	Calls *CallContainer
	Fail  bool
}

func (h TestBaseplateHook) OnServerSpanCreate(span *tracing.Span) error {
	span.RegisterHook(TestSpanHook{Calls: h.Calls, Fail: h.Fail})
	return h.Calls.AddCall("on-server-span-create", h.Fail)
}

type TestSpanHook struct {
	Calls *CallContainer
	Fail  bool
}

func (h TestSpanHook) OnCreateChild(child *tracing.Span) error {
	return h.Calls.AddCall("on-create-child", h.Fail)
}

func (h TestSpanHook) OnStart(child *tracing.Span) error {
	return h.Calls.AddCall("on-start", h.Fail)
}

func (h TestSpanHook) OnEnd(child *tracing.Span, err error) error {
	return h.Calls.AddCall("on-end", h.Fail)
}

func (h TestSpanHook) OnSetTag(span *tracing.Span, key string, value interface{}) error {
	return h.Calls.AddCall("on-set-tag", h.Fail)
}

func (h TestSpanHook) OnAddCounter(span *tracing.Span, key string, delta float64) error {
	return h.Calls.AddCall("on-add-counter", h.Fail)
}

var (
	_ tracing.BaseplateHook = TestBaseplateHook{}
	_ tracing.SpanHook      = TestSpanHook{}
)

func TestHooks(t *testing.T) {
	hook := TestBaseplateHook{Calls: &CallContainer{}, Fail: false}
	tracing.RegisterBaseplateHook(hook)
	defer tracing.ResetHooks()

	ctx := context.Background()
	span := tracing.StartSpanFromThriftContext(ctx, "foo")
	span.SetTag("foo", "bar")
	span.CreateClientChild("bar")
	span.AddCounter("foo", 1.0)
	span.End(ctx, nil)
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

func TestHookFailures(t *testing.T) {
	hook := TestBaseplateHook{Calls: &CallContainer{}, Fail: true}
	tracing.RegisterBaseplateHook(hook)
	defer tracing.ResetHooks()

	ctx := context.Background()
	span := tracing.StartSpanFromThriftContext(ctx, "foo")
	span.SetTag("foo", "bar")
	span.CreateClientChild("bar")
	span.AddCounter("foo", 1.0)
	span.End(ctx, nil)
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
