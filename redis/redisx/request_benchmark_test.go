package redisx

import (
	"reflect"
	"testing"
)

// goos: darwin
// goarch: amd64
// pkg: github.com/reddit/baseplate.go/redis/redisx
// BenchmarkSetValue
// BenchmarkSetValue/string
// BenchmarkSetValue/string-12         	14946445	        67.1 ns/op
// BenchmarkSetValue/int64
// BenchmarkSetValue/int64-12          	31571834	        34.5 ns/op
// BenchmarkSetValue/[]byte
// BenchmarkSetValue/[]byte-12         	13296234	        82.4 ns/op
// BenchmarkSetValue/[]interface{}
// BenchmarkSetValue/[]interface{}-12  	10814821	       105 ns/op
func BenchmarkSetValue(b *testing.B) {
	b.Run("string", func(b *testing.B) {
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

	b.Run("int64", func(b *testing.B) {
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

	b.Run("[]byte", func(b *testing.B) {
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
// pkg: github.com/reddit/baseplate.go/redis/redisx
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
// pkg: github.com/reddit/baseplate.go/redis/redisx
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
