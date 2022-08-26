package ecinterface

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestGlobal_doesntPanic(t *testing.T) {
	Get()
}

func BenchmarkAtomics(b *testing.B) {
	// Example results:
	//
	// BenchmarkAtomics/rwmutex-10         	    85623068	    13.76 ns/op
	// BenchmarkAtomics/atomicValue-10     	    283137508	     4.204 ns/op
	// BenchmarkAtomics/atomicStructValue-10    703642080	     1.705 ns/op
	// BenchmarkAtomics/atomicPointer-10        568728128	     2.070 ns/op
	b.Run("current", func(b *testing.B) {
		Set(nop{})
		b.ReportAllocs()
		b.ResetTimer()
		var count int
		for i := 0; i < b.N; i++ {
			if Get() != nil {
				count++
			}
		}
		if count != b.N {
			b.Fatalf("this is just to avoid eliding the call")
		}
	})
	b.Run("rwmutex", func(b *testing.B) {
		var scope struct {
			rw sync.RWMutex
			Interface
		}
		scope.Interface = nop{}
		get := func() Interface {
			scope.rw.RLock()
			defer scope.rw.RUnlock()
			return scope.Interface
		}
		b.ReportAllocs()
		b.ResetTimer()
		var count int
		for i := 0; i < b.N; i++ {
			if get() != nil {
				count++
			}
		}
		if count != b.N {
			b.Fatalf("this is just to avoid eliding the call")
		}
	})
	b.Run("atomicValue", func(b *testing.B) {
		var global atomic.Value
		global.Store(nop{})
		b.ReportAllocs()
		b.ResetTimer()
		var count int
		for i := 0; i < b.N; i++ {
			if global.Load().(Interface) != nil {
				count++
			}
		}
		if count != b.N {
			b.Fatalf("this is just to avoid eliding the call")
		}
	})
	b.Run("atomicStructValue", func(b *testing.B) {
		var global atomic.Value
		type current struct{ Interface }
		global.Store(current{nop{}})
		b.ReportAllocs()
		b.ResetTimer()
		var count int
		for i := 0; i < b.N; i++ {
			if global.Load().(current).Interface != nil {
				count++
			}
		}
		if count != b.N {
			b.Fatalf("this is just to avoid eliding the call")
		}
	})
	/* If you have a go1.18 compiler, you can throw this one in for fun:
	b.Run("atomicPointer", func(b *testing.B) {
		var global atomic.Pointer[Interface]
		var impl Interface = nop{}
		global.Store(&impl)
		b.ReportAllocs()
		b.ResetTimer()
		var count int
		for i := 0; i < b.N; i++ {
			if *global.Load() != nil {
				count++
			}
		}
		if count != b.N {
			b.Fatalf("this is just to avoid eliding the call")
		}
	})
	*/
}
