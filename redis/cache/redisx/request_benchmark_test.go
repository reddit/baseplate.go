package redisx

import (
	"reflect"
	"testing"
)

// goos: linux
// goarch: amd64
// pkg: github.com/reddit/baseplate.go/redis/cache/redisx
// cpu: AMD EPYC-Rome Processor
// BenchmarkSetValue/string_string-8               14308600                82.94 ns/op
// BenchmarkSetValue/int64_int64-8                 27027405                42.59 ns/op
// BenchmarkSetValue/[]byte_[]byte-8               10459291               106.0 ns/op
// BenchmarkSetValue/[]byte_string-8                5159313               233.9 ns/op
// BenchmarkSetValue/[]byte_*string-8               5864259               207.7 ns/op
// BenchmarkSetValue/[]byte_int64-8                 6185367               210.9 ns/op
// BenchmarkSetValue/[]byte_*int64-8                4815808               267.4 ns/op
// BenchmarkSetValue/[]interface{}-8                9794007               118.6 ns/op
func BenchmarkSetValue(b *testing.B) {
	b.Run("string_string", func(b *testing.B) {
		var v string
		r := Req(&v, "PING")
		res := "foo"

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := r.setValue(res); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("int64_int64", func(b *testing.B) {
		var v int64
		r := Req(&v, "INCR", "foo")
		var res int64 = 123

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := r.setValue(res); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("[]byte_[]byte", func(b *testing.B) {
		var v []byte
		r := Req(&v, "GET", "foo")
		res := []byte("foo")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := r.setValue(res); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("[]byte_string", func(b *testing.B) {
		var v string
		r := Req(&v, "GET", "foo")
		res := []byte("foo")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := r.setValue(res); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("[]byte_*string", func(b *testing.B) {
		var v *string
		r := Req(&v, "GET", "foo")
		res := []byte("foo")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := r.setValue(res); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("[]byte_int64", func(b *testing.B) {
		var v int64
		r := Req(&v, "GET", "foo")
		res := []byte("123")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := r.setValue(res); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("[]byte_*int64", func(b *testing.B) {
		var v *int64
		r := Req(&v, "GET", "foo")
		res := []byte("123")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := r.setValue(res); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("[]interface{}", func(b *testing.B) {
		var v []interface{}
		r := Req(&v, "MGET", "foo", "bar")
		res := []interface{}{[]byte("foo"), []byte("bar")}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := r.setValue(res); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// goos: darwin
// goarch: amd64
// pkg: github.com/reddit/baseplate.go/redis/cache/redisx
// BenchmarkCachedStructFields
// BenchmarkCachedStructFields-12    	40746420	        29.8 ns/op
func BenchmarkCachedStructFields(b *testing.B) {
	type testStruct struct {
		Foo  int64
		Bar  string
		Fizz []byte
		Buzz []byte
	}
	t := reflect.TypeOf(testStruct{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cachedStructFields(t)
	}
}

// goos: darwin
// goarch: amd64
// pkg: github.com/reddit/baseplate.go/redis/cache/redisx
// BenchmarkGetStructFields
// BenchmarkGetStructFields-12    	 2092642	       571 ns/op
func BenchmarkGetStructFields(b *testing.B) {
	type testStruct struct {
		Foo  int64
		Bar  string
		Fizz []byte
		Buzz []byte
	}
	t := reflect.TypeOf(testStruct{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getStructFields(t)
	}
}
