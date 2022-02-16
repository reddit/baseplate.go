package redisx_test

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/joomcode/errorx"
	"github.com/joomcode/redispipe/redis"

	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/redis/cache/redisx"
)

const (
	pong = "PONG"
	ok   = "OK"
)

func TestSyncx_Do(t *testing.T) {
	defer flushRedis()

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		var v string
		if err := client.Do(ctx, &v, "PING"); err != nil {
			t.Fatal(err)
		}
		if v != pong {
			t.Errorf("wrong response, expected %q, got %q", pong, v)
		}
	})

	t.Run("error/command", func(t *testing.T) {
		var v string
		if err := client.Do(ctx, &v, "FOO"); err == nil {
			t.Fatal("expected an error, got nil")
		}
		if v != "" {
			t.Errorf("expected v to be empty, got %q", v)
		}
	})

	t.Run("error/context", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		var v string
		if err := client.Do(cancelCtx, &v, "PING"); err == nil {
			t.Fatal("expected an error, got nil")
		}
		if v != "" {
			t.Errorf("expected v to be empty, got %q", v)
		}
	})
}

func TestSyncx_Send(t *testing.T) {
	defer flushRedis()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		var v string
		if err := client.Send(ctx, redisx.Req(&v, "PING")); err != nil {
			t.Fatal(err)
		}
		if v != pong {
			t.Errorf("wrong response, expected %q, got %q", pong, v)
		}
	})

	t.Run("error/command", func(t *testing.T) {
		var v string
		if err := client.Send(ctx, redisx.Req(&v, "FOO")); err == nil {
			t.Fatal("expected an error, got nil")
		}
		if v != "" {
			t.Errorf("expected v to be empty, got %q", v)
		}
	})

	t.Run("error/context", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		var v string
		if err := client.Send(cancelCtx, redisx.Req(&v, "PING")); err == nil {
			t.Fatal("expected an error, got nil")
		}
		if v != "" {
			t.Errorf("expected v to be empty, got %q", v)
		}
	})
}

func TestSyncx_SendMany(t *testing.T) {
	defer flushRedis()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		var v1 string
		var v2 string
		errs := client.SendMany(ctx,
			redisx.Req(&v1, "PING"),
			redisx.Req(&v2, "SET", "k", "v"),
		)
		batch := errorsbp.Batch{}
		batch.Add(errs...)
		if batch.Compile() != nil {
			t.Fatalf("expected no errors, got %+v", errs)
		}
		if v1 != pong {
			t.Errorf("v1 did not match, expected %q, got %q", pong, v1)
		}
		if v2 != ok {
			t.Errorf("v2 did not match, expected %q, got %q", ok, v2)
		}
	})

	t.Run("error/command", func(t *testing.T) {
		if err := client.Do(ctx, nil, "FLUSHALL"); err != nil {
			t.Fatal(err)
		}

		var v string
		errs := client.SendMany(ctx,
			redisx.Req(nil, "FOO"),
			redisx.Req(nil, "BAR"),
			redisx.Req(&v, "SET", "x", "y"),
		)
		if errs[0] == nil {
			t.Error("expected first command to return an error, got nil.")
		}
		if errs[1] == nil {
			t.Error("expected second command to return an error, got nil.")
		}
		if errs[2] != nil {
			t.Errorf("expected third command to not return an error, got %+v.", errs[2])
		}
		if v != ok {
			t.Errorf("v did not match, expected %q, got %q", ok, v)
		}
	})

	t.Run("error/context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()

		errs := client.SendMany(ctx,
			redisx.Req(nil, "PING"),
			redisx.Req(nil, "SET", "k", "v"),
		)
		for i, err := range errs {
			var e *errorx.Error
			if errors.As(err, &e) {
				if !e.IsOfType(redis.ErrRequestCancelled) {
					t.Errorf("expected command %d to return an redis.ErrRequestCancelled, got %+v.", i+1, err)
				}
			} else {
				t.Errorf("expected command %d to return an errorx.Error, got %+v.", i+1, err)
			}
		}
	})
}

func TestMonitoredSync_SendTransaction(t *testing.T) {
	defer flushRedis()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		var v1 string
		var v2 string
		if err := client.SendTransaction(ctx,
			redisx.Req(&v1, "SET", "x", "y"),
			redisx.Req(&v2, "SET", "k", "v"),
		); err != nil {
			t.Fatal(err)
		}
		if v1 != ok {
			t.Errorf("v1 did not match, expected %q, got %q", ok, v1)
		}
		if v2 != ok {
			t.Errorf("v2 did not match, expected %q, got %q", ok, v2)
		}
	})

	t.Run("error/context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()

		if err := client.SendTransaction(ctx,
			redisx.Req(nil, "PING"),
			redisx.Req(nil, "SET", "k", "v"),
		); err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}

func TestSyncx_Scanner(t *testing.T) {
	defer flushRedis()

	ctx := context.Background()

	errs := client.SendMany(ctx,
		redisx.Req(nil, "SET", "foo:x", "x"),
		redisx.Req(nil, "SET", "foo:y", "y"),
		redisx.Req(nil, "SET", "bar:x", "a"),
		redisx.Req(nil, "SET", "bar:y", "b"),
	)

	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	scanner := client.Scanner(ctx, redis.ScanOpts{Match: "foo:*"})
	_, err := scanner.Next()
	if err != nil {
		t.Fatal(err)
	}
}

func TestResponseTypes(t *testing.T) {
	defer flushRedis()

	ctx := context.Background()

	t.Run("int64", func(t *testing.T) {
		key := "foo"
		var v int64
		if err := client.Do(ctx, &v, "INCR", key); err != nil {
			t.Fatal(err)
		}
		if v != 1 {
			t.Errorf("expected value to be 1, got %d", v)
		}
	})

	t.Run("string", func(t *testing.T) {
		var v string
		if err := client.Do(ctx, &v, "PING"); err != nil {
			t.Fatal(err)
		}
		if v != pong {
			t.Errorf("string value mismatch, expected %q, got %q", pong, v)
		}
	})

	t.Run("*string", func(t *testing.T) {
		key := "star"

		var v *string
		if err := client.Do(ctx, &v, "GET", key); err != nil {
			t.Fatal(err)
		}
		if v != nil {
			t.Errorf("expected value to not be set, got %v", v)
		}

		value := "buzz"
		if err := client.Do(ctx, nil, "SET", key, value); err != nil {
			t.Fatal(err)
		}

		if err := client.Do(ctx, &v, "GET", key); err != nil {
			t.Fatal(err)
		}
		if *v != value {
			t.Errorf("*string value mismatch, expected %q, got %v", value, v)
		}

	})

	t.Run("[]byte", func(t *testing.T) {
		key := "fizz"

		var v []byte
		if err := client.Do(ctx, &v, "GET", key); err != nil {
			t.Fatal(err)
		}
		if v != nil {
			t.Errorf("expected value to not be set, got %q", string(v))
		}

		value := "buzz"
		if err := client.Do(ctx, nil, "SET", key, value); err != nil {
			t.Fatal(err)
		}

		t.Run("input/[]byte", func(t *testing.T) {
			var v []byte
			if err := client.Do(ctx, &v, "GET", key); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(v, []byte(value)) {
				t.Errorf("bulk string value mismatch, expected %q, got %q", value, string(v))
			}
		})

		t.Run("input/string", func(t *testing.T) {
			var v string
			if err := client.Do(ctx, &v, "GET", key); err != nil {
				t.Fatal(err)
			}
			if v != value {
				t.Errorf("bulk string value mismatch, expected %q, got %q", value, string(v))
			}
		})

		t.Run("input/int64", func(t *testing.T) {
			var value int64 = 3
			if err := client.Do(ctx, nil, "SET", key, strconv.FormatInt(value, 10)); err != nil {
				t.Fatal(err)
			}

			var v int64
			if err := client.Do(ctx, &v, "GET", key); err != nil {
				t.Fatal(err)
			}
			if v != value {
				t.Errorf("bulk string value mismatch, expected %d, got %d", value, v)
			}
		})
	})

	t.Run("[]interface{}", func(t *testing.T) {
		const (
			key1 = "alpha"
			val1 = "foo"

			key2 = "omega"
			val2 = "bar"

			key3 = "integer"
			val3 = 3

			key4 = "missing"
		)

		var v []interface{}
		if err := client.Do(ctx, &v, "MGET", key1, key2); err != nil {
			t.Fatal(err)
		}
		nilResp := []interface{}{nil, nil}
		if !reflect.DeepEqual(v, nilResp) {
			t.Errorf("array response mismatch, expected %+v, got %+v", nilResp, v)
		}

		if err := client.Do(ctx, nil, "MSET", key1, val1, key2, val2); err != nil {
			t.Fatal(err)
		}
		if err := client.Do(ctx, nil, "INCRBY", key3, val3); err != nil {
			t.Fatal(err)
		}

		if err := client.Do(ctx, &v, "MGET", key1, key2, key3, key4); err != nil {
			t.Fatal(err)
		}
		expected := []interface{}{[]byte(val1), []byte(val2), []byte("3"), nil}
		if !reflect.DeepEqual(v, expected) {
			t.Errorf("array response mismatch, expected %+v, got %+v", expected, v)
		}
	})
}

func TestArrayCommandResponse_SliceLiteral(t *testing.T) {
	defer flushRedis()
	ctx := context.Background()

	const (
		val1 = "foo"
		val2 = "bar"
		key  = "key"
	)

	t.Run("[][]byte", func(t *testing.T) {
		cases := []struct {
			cmd      string
			setup    func() error
			args     []interface{}
			expected [][]byte
		}{
			{
				cmd: "LRANGE",
				setup: func() error {
					return client.Do(ctx, nil, "LPUSH", key, val2, val1)
				},
				args:     []interface{}{key, 0, -1},
				expected: [][]byte{[]byte(val1), []byte(val2)},
			},
		}
		for _, _c := range cases {
			c := _c
			t.Run(c.cmd, func(t *testing.T) {
				defer flushRedis()
				if err := c.setup(); err != nil {
					t.Fatal(err)
				}
				var v [][]byte
				if err := client.Do(ctx, &v, c.cmd, c.args...); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(v, c.expected) {
					t.Errorf("array response mismatch, expected %+v, got %+v", c.expected, v)
				}
			})
		}
	})

	t.Run("[]int64", func(t *testing.T) {
		cases := []struct {
			cmd      string
			setup    func() error
			args     []interface{}
			expected []int64
		}{
			{
				cmd:   "BITFIELD",
				setup: func() error { return nil },
				// Example command from https://redis.io/commands/bitfield
				args:     []interface{}{key, "INCRBY", "i5", 100, 1, "GET", "u4", 0},
				expected: []int64{1, 0},
			},
		}
		for _, _c := range cases {
			c := _c
			t.Run(c.cmd, func(t *testing.T) {
				defer flushRedis()

				if err := c.setup(); err != nil {
					t.Fatal(err)
				}
				var v []int64
				if err := client.Do(ctx, &v, c.cmd, c.args...); err != nil {
					t.Skip("miniredis does not support the BITFIELD command.")
				}
				if !reflect.DeepEqual(v, c.expected) {
					t.Errorf("array response mismatch, expected %+v, got %+v", c.expected, v)
				}
			})
		}
	})
}

func TestArrayCommandResponse_StructScanning(t *testing.T) {
	defer flushRedis()
	ctx := context.Background()

	const (
		key1 = "alpha"
		val1 = "foo"

		key2 = "omega"
		val2 = "bar"

		key3 = "integer"
		val3 = 3

		key4 = "missing"
	)

	if err := client.Do(ctx, nil, "MSET", key1, val1, key2, val2); err != nil {
		t.Fatal(err)
	}
	if err := client.Do(ctx, nil, "INCRBY", key3, val3); err != nil {
		t.Fatal(err)
	}

	t.Run("basic", func(t *testing.T) {

		type response struct {
			Alpha   []byte
			Omega   []byte
			Integer []byte
			Missing []byte
		}

		var v response
		if err := client.Do(ctx, &v, "MGET", key1, key2, key3, key4); err != nil {
			t.Fatal(err)
		}
		expectedStruct := response{
			Alpha:   []byte(val1),
			Omega:   []byte(val2),
			Integer: []byte("3"),
		}
		if !reflect.DeepEqual(v, expectedStruct) {
			t.Errorf("array response mismatch, expected %+v, got %+v", expectedStruct, v)
		}
	})

	t.Run("complex", func(t *testing.T) {

		type response struct {
			Alpha   []byte
			Omega   string
			Integer int64
			Missing []byte
		}

		var v response
		if err := client.Do(ctx, &v, "MGET", key1, key2, key3, key4); err != nil {
			t.Fatal(err)
		}
		expectedStruct := response{
			Alpha:   []byte(val1),
			Omega:   val2,
			Integer: 3,
		}
		if !reflect.DeepEqual(v, expectedStruct) {
			t.Errorf("array response mismatch, expected %+v, got %+v", expectedStruct, v)
		}
	})

	t.Run("tags", func(t *testing.T) {

		type response struct {
			Foo   []byte `redisx:"alpha"`
			Bar   string `redisx:"omega"`
			Fizz  int64  `redisx:"integer"`
			Buzz  []byte `redisx:"missing"`
			Alpha []byte `redisx:"-"`
		}

		var v response
		if err := client.Do(ctx, &v, "MGET", key1, key2, key3, key4); err != nil {
			t.Fatal(err)
		}
		expectedStruct := response{
			Foo:  []byte(val1),
			Bar:  val2,
			Fizz: 3,
		}
		if !reflect.DeepEqual(v, expectedStruct) {
			t.Errorf("array response mismatch, expected %+v, got %+v", expectedStruct, v)
		}
	})
}

type testErrorsCase struct {
	name  string
	setup func(ctx context.Context, args []interface{}) error
	v     interface{}
	cmd   string
	args  []interface{}
}

func TestErrors_InvalidInput(t *testing.T) {
	defer flushRedis()

	const key = "key"

	setupGet := func(ctx context.Context, args []interface{}) error {
		key := args[0].(string)
		return client.Do(ctx, nil, "SET", key, "foo")
	}

	var (
		inputInt64 int64
		inputInt32 int32
	)

	ctx := context.Background()
	cases := []testErrorsCase{
		{
			name:  "not-pointer",
			setup: setupGet,
			v:     "",
			cmd:   "GET",
			args:  []interface{}{key},
		},
		{
			name:  "pointer-to-nil",
			setup: setupGet,
			v:     (*string)(nil),
			cmd:   "GET",
			args:  []interface{}{key},
		},
		{
			name:  "unsupported-type",
			setup: setupGet,
			v:     &inputInt32,
			cmd:   "GET",
			args:  []interface{}{key},
		},
		{
			name:  "bulk-string-conversion-type-mismatch/literal",
			setup: setupGet,
			v:     &inputInt64,
			cmd:   "GET",
			args:  []interface{}{key},
		},
		{
			name:  "bulk-string-conversion-type-mismatch/struct",
			setup: setupGet,
			v:     &struct{ Key int64 }{},
			cmd:   "MGET",
			args:  []interface{}{key},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(c.name, func(t *testing.T) {
			defer flushRedis()

			if err := c.setup(ctx, c.args); err != nil {
				t.Fatal(err)
			}
			var expected *redisx.InvalidInputError

			err := client.Do(ctx, c.v, c.cmd, c.args...)
			if err == nil {
				t.Fatal("expected an error, got nil")
			} else if !errors.As(err, &expected) {
				t.Fatalf("error mismatch, expected %T, got %+v", expected, err)
			}
		})
	}
}

func TestErrors_ResponseInputTypeError(t *testing.T) {
	defer flushRedis()

	const key = "key"

	var (
		bytesArrayInput [][]byte
		byteArrayInput  []byte
		stringInput     string
		int64Input      int64
		int64ArrayInput []int64
		structInput     struct {
			Key []byte
		}
	)

	setupGet := func(ctx context.Context, args []interface{}) error {
		key := args[0].(string)
		return client.Do(ctx, nil, "SET", key, "foo")
	}

	setupLRange := func(ctx context.Context, args []interface{}) error {
		key := args[0].(string)
		return client.Do(ctx, nil, "RPUSH", key, "foo", "bar")
	}

	ctx := context.Background()
	cases := []testErrorsCase{
		// Test array commands that support a slice of int64s
		{
			name: "[]int64/[]byte",
			cmd:  "BITFIELD",
			v:    &byteArrayInput,
			args: []interface{}{key, "INCRBY", "i5", 100, 1, "GET", "u4", 0},
		},
		{
			name: "[]int64/[][]byte",
			cmd:  "BITFIELD",
			v:    &bytesArrayInput,
			args: []interface{}{key, "INCRBY", "i5", 100, 1, "GET", "u4", 0},
		},
		{
			name: "[]int64/int64",
			cmd:  "BITFIELD",
			v:    &int64Input,
			args: []interface{}{key, "INCRBY", "i5", 100, 1, "GET", "u4", 0},
		},
		{
			name: "[]int64/string",
			cmd:  "BITFIELD",
			v:    &stringInput,
			args: []interface{}{key, "INCRBY", "i5", 100, 1, "GET", "u4", 0},
		},
		{
			name: "[]int64/struct",
			cmd:  "BITFIELD",
			v:    &structInput,
			args: []interface{}{key, "INCRBY", "i5", 100, 1, "GET", "u4", 0},
		},

		// Test array commands that support struct scanning
		{
			name:  "struct/[]byte",
			cmd:   "MGET",
			v:     &byteArrayInput,
			args:  []interface{}{key},
			setup: setupGet,
		},
		{
			name:  "struct/[][]byte",
			cmd:   "MGET",
			v:     &bytesArrayInput,
			args:  []interface{}{key},
			setup: setupGet,
		},
		{
			name:  "struct/int64",
			cmd:   "MGET",
			v:     &int64Input,
			args:  []interface{}{key},
			setup: setupGet,
		},
		{
			name:  "struct/[]int64",
			cmd:   "MGET",
			v:     &int64ArrayInput,
			args:  []interface{}{key},
			setup: setupGet,
		},
		{
			name:  "struct/string",
			cmd:   "MGET",
			v:     &stringInput,
			args:  []interface{}{key},
			setup: setupGet,
		},
		{
			name: "struct/struct-with-unsupported-type",
			cmd:  "MGET",
			v: &struct {
				Key int32
			}{},
			args:  []interface{}{key},
			setup: setupGet,
		},

		// Test array commands that support a slice of byte slices
		{
			name:  "[][]byte/[]byte",
			cmd:   "LRANGE",
			v:     &byteArrayInput,
			args:  []interface{}{key, 0, -1},
			setup: setupLRange,
		},
		{
			name:  "[][]byte/int64",
			cmd:   "LRANGE",
			v:     &int64Input,
			args:  []interface{}{key, 0, -1},
			setup: setupLRange,
		},
		{
			name:  "[][]byte/[]int64",
			cmd:   "LRANGE",
			v:     &int64ArrayInput,
			args:  []interface{}{key, 0, -1},
			setup: setupLRange,
		},
		{
			name:  "[][]byte/string",
			cmd:   "LRANGE",
			v:     &stringInput,
			args:  []interface{}{key, 0, -1},
			setup: setupLRange,
		},
		{
			name:  "[][]byte/struct",
			cmd:   "LRANGE",
			v:     &structInput,
			args:  []interface{}{key, 0, -1},
			setup: setupLRange,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(c.name, func(t *testing.T) {
			if c.cmd == "BITFIELD" {
				t.Skip("skipping since miniredis does not support the BITFIELD command.")
			}
			defer flushRedis()

			if c.setup != nil {
				if err := c.setup(ctx, c.args); err != nil {
					t.Fatal(err)
				}
			}

			var expected *redisx.ResponseInputTypeError

			err := client.Do(ctx, c.v, c.cmd, c.args...)
			if err == nil {
				t.Fatal("expected an error, got nil")
			} else if !errors.As(err, &expected) {
				t.Fatalf("error mismatch, expected %T, got %+v", expected, err)
			}
		})
	}
}
