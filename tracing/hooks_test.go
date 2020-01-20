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

func (c *CallContainer) AddCall(call string) {
	c.Calls = append(c.Calls, call)
}

func (c *CallContainer) Reset() {
	c.Calls = nil
}

type TestBaseplateHook struct {
	Calls *CallContainer
	Fail  bool
}

func (h TestBaseplateHook) OnServerSpanCreate(span *tracing.Span) error {
	label := "on-server-span-create"
	h.Calls.AddCall(label)
	span.RegisterHook(TestSpanHook{Calls: h.Calls, Fail: h.Fail})
	if h.Fail {
		return fmt.Errorf(label)
	}
	return nil
}

type TestSpanHook struct {
	Calls *CallContainer
	Fail  bool
}

func (h TestSpanHook) OnCreateChild(child *tracing.Span) error {
	label := "on-create-child"
	h.Calls.AddCall(label)
	if h.Fail {
		return fmt.Errorf(label)
	}
	return nil
}

func (h TestSpanHook) OnStart(child *tracing.Span) error {
	label := "on-start"
	h.Calls.AddCall(label)
	if h.Fail {
		return fmt.Errorf(label)
	}
	return nil
}

func (h TestSpanHook) OnEnd(child *tracing.Span, err error) error {
	label := "on-end"
	h.Calls.AddCall(label)
	if h.Fail {
		return fmt.Errorf(label)
	}
	return nil
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
	span.CreateClientChild("bar")
	span.End(ctx, nil)
	expected := []string{
		"on-server-span-create",
		"on-start",
		"on-create-child",
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
	span.CreateClientChild("bar")
	span.End(ctx, nil)
	expected := []string{
		"on-server-span-create",
		"on-start",
		"on-create-child",
		"on-end",
	}
	if !reflect.DeepEqual(hook.Calls.Calls, expected) {
		t.Fatalf("Expected %v:\nGot: %v", expected, hook.Calls.Calls)
	}
}
