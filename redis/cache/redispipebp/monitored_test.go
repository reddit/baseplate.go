package redispipebp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/joomcode/redispipe/redis"

	"github.com/reddit/baseplate.go/redis/cache/redispipebp"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	pong = "PONG"
	okay = "OK"
	name = "monitored-redis"

	testTimeout = 100 * time.Millisecond
)

func checkSpans(t *testing.T, name string, errExpected bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	msg, err := mmq.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var trace tracing.ZipkinSpan
	err = json.Unmarshal(msg, &trace)
	if err != nil {
		t.Fatal(err)
	}
	if len(trace.BinaryAnnotations) == 0 {
		t.Error("no binary annotations")
	}
	hasError := false
	for _, annotation := range trace.BinaryAnnotations {
		if annotation.Key == "error" {
			hasError = true
		}
	}
	if hasError != errExpected {
		t.Errorf("error expected: %v, has error: %v", errExpected, hasError)
	}
	if trace.Name != name {
		t.Errorf("name mismatch, expectd %q, got %q", name, trace.Name)
	}
}

func TestMonitoredSync_Do(t *testing.T) {
	defer flushRedis()

	mClient := redispipebp.MonitoredSync{
		Sync: client,
		Name: name,
	}

	ctx := context.Background()
	name := mClient.Name + ".do"

	t.Run("success", func(t *testing.T) {
		r := mClient.Do(ctx, "PING")
		if resp, ok := r.(string); !ok {
			t.Fatalf("wrong response type %T for %+v, expected %T", r, r, resp)
		} else if resp != pong {
			t.Errorf("wrong response, expected %q, got %q", pong, resp)
		}
		checkSpans(t, name, false)
	})

	t.Run("error/command", func(t *testing.T) {
		r := mClient.Do(ctx, "FOO")
		if err := redis.AsError(r); err == nil {
			t.Fatalf("expected response to be an error, got %+v", r)
		}
		checkSpans(t, name, true)
	})

	t.Run("error/context", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		r := mClient.Do(cancelCtx, "PING")
		if err := redis.AsError(r); err == nil {
			t.Fatalf("expected response to be an error, got %+v", r)
		}
		checkSpans(t, name, true)
	})
}

func TestMonitoredSync_Send(t *testing.T) {
	defer flushRedis()

	mClient := redispipebp.MonitoredSync{
		Sync: client,
		Name: name,
	}

	ctx := context.Background()
	name := mClient.Name + ".send"

	t.Run("success", func(t *testing.T) {
		r := mClient.Send(ctx, redis.Req("PING"))
		if resp, ok := r.(string); !ok {
			t.Fatalf("wrong response type %T for %+v, expected %T", r, r, resp)
		} else if resp != pong {
			t.Errorf("wrong response, expected %q, got %q", pong, resp)
		}
		checkSpans(t, name, false)
	})

	t.Run("error/command", func(t *testing.T) {
		r := mClient.Send(ctx, redis.Req("FOO"))
		if err := redis.AsError(r); err == nil {
			t.Fatalf("expected response to be an error, got %+v", r)
		}
		checkSpans(t, name, true)
	})

	t.Run("error/context", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		r := mClient.Send(cancelCtx, redis.Req("PING"))
		if err := redis.AsError(r); err == nil {
			t.Fatalf("expected response to be an error, got %+v", r)
		}
		checkSpans(t, name, true)
	})
}

func TestMonitoredSync_SendMany(t *testing.T) {
	defer flushRedis()

	mClient := redispipebp.MonitoredSync{
		Sync: client,
		Name: name,
	}

	ctx := context.Background()
	name := mClient.Name + ".send-many"

	t.Run("success", func(t *testing.T) {
		r := mClient.SendMany(ctx, []redis.Request{
			redis.Req("PING"),
			redis.Req("SET", "k", "v"),
		})
		for i, expected := range []string{pong, okay} {
			val := r[i]
			if resp, ok := val.(string); !ok {
				t.Fatalf("wrong response type %T for %+v, expected %T", val, val, resp)
			} else if resp != expected {
				t.Errorf("wrong response, expected %q, got %q", expected, resp)
			}
		}
		checkSpans(t, name, false)
	})

	t.Run("error/command", func(t *testing.T) {
		mClient.SendMany(ctx, []redis.Request{
			redis.Req("FOO"),
			redis.Req("BAR"),
			redis.Req("SET", "x", "y"),
		})
		// Only redis.ErrRequestCancelled errors are reported with SendMany.
		checkSpans(t, name, false)
	})

	t.Run("error/context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()

		mClient.SendMany(ctx, []redis.Request{
			redis.Req("PING"),
			redis.Req("SET", "k", "v"),
		})
		checkSpans(t, name, true)
	})
}

func TestMonitoredSync_SendTransaction(t *testing.T) {
	defer flushRedis()

	mClient := redispipebp.MonitoredSync{
		Sync: client,
		Name: name,
	}

	ctx := context.Background()
	name := mClient.Name + ".send-transaction"

	t.Run("success", func(t *testing.T) {
		r, err := mClient.SendTransaction(ctx, []redis.Request{
			redis.Req("SET", "x", "y"),
			redis.Req("SET", "k", "v"),
		})
		for i, expected := range []string{okay, okay} {
			val := r[i]
			if resp, ok := val.(string); !ok {
				t.Errorf("wrong response type %T for %+v, expected %T", val, val, resp)
			} else if resp != expected {
				t.Errorf("wrong response, expected %q, got %q", expected, resp)
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		checkSpans(t, name, false)
	})

	t.Run("error/command", func(t *testing.T) {
		t.Skip(
			"miniredis does not handle transactions correctly, an error in a transaction does not cause the entire transaction to fail",
		)

		if _, err := mClient.SendTransaction(ctx, []redis.Request{
			redis.Req("FOO"),
			redis.Req("SET", "x", "y"),
		}); err == nil {
			t.Fatal("expected an error ,got nil")
		}
		checkSpans(t, name, true)
	})

	t.Run("error/context", func(t *testing.T) {
		t.Skip(
			"miniredis does not handle transactions correctly, an error in a transaction does not cause the entire transaction to fail",
		)

		ctx, cancel := context.WithCancel(ctx)
		cancel()

		if _, err := mClient.SendTransaction(ctx, []redis.Request{
			redis.Req("PING"),
			redis.Req("SET", "k", "v"),
		}); err == nil {
			t.Fatal("expected an error, got nil")
		}
		checkSpans(t, name, true)
	})
}

func TestMonitoredSync_Scanner(t *testing.T) {
	defer flushRedis()

	mClient := redispipebp.MonitoredSync{
		Sync: client,
		Name: name,
	}

	ctx := context.Background()
	r := mClient.SendMany(ctx, []redis.Request{
		redis.Req("SET", "foo:x", "x"),
		redis.Req("SET", "foo:y", "y"),
		redis.Req("SET", "bar:x", "a"),
		redis.Req("SET", "bar:y", "b"),
	})

	for _, resp := range r {
		if err := redis.AsError(resp); err != nil {
			t.Fatal(err)
		}
	}
	checkSpans(t, mClient.Name+".send-many", false)

	scanner := mClient.Scanner(ctx, redis.ScanOpts{Match: "foo:*"})
	if _, err := scanner.Next(); err != nil {
		t.Fatal(err)
	}
	checkSpans(t, mClient.Name+".scanner.next", false)
}
