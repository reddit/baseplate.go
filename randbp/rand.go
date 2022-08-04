package randbp

import (
	"math/rand"
	"sync"
)

var readerPool = sync.Pool{
	New: func() interface{} {
		return rand.New(rand.NewSource(1))
	},
}

// R is a global thread-safe rng.
//
// It embeds *math/rand.Rand, but properly seeded and safe for concurrent use.
//
// It should be used instead of the global functions inside math/rand package.
//
// For example, instead of this:
//
//	import "math/rand"
//	i := rand.Uint64()
//
// Use this:
//
//	import "github.com/reddit/baseplate.go/randbp"
//	i := randbp.R.Uint64()
//
// NOTE: Its Read function has worse performance comparing to rand's global
// rander or rand.Rand created with non-thread-safe source for small buffers.
// See the doc of Rand.Read for more details.
// All other functions (Uint64, Float64, etc.) have comparable performance to
// math/rand's implementations.
var R = New(GetSeed())

// Rand embeds *math/rand.Rand.
//
// All functions besides Read are directly calling the embedded rand.Rand.
// When initialized with New(), all functions are safe for concurrent use,
// and have comparable performance to the top level math/rand functions.
//
// See the doc of Read function for more details on that one.
type Rand struct {
	*rand.Rand
}

// Read overrides math/rand's Read implementation with thread-safety.
//
// It's safe for concurrent use and always returns len(p) with nil error.
//
// Compare to math/rand.Read (the top level one) performance-wise,
// it has a constant ~1us overhead,
// which is significant when len(p) is small,
// but less significant when len(p) grows larger,
// and starts to outperform math/rand.Read when len(p) is very large because it
// only need to lock once.
// See the following sample benchmark result:
//
//	$ go test -bench Rand/Read -benchmem
//	goos: darwin
//	goarch: amd64
//	pkg: github.com/reddit/baseplate.go/randbp
//	BenchmarkRand/Read/size-16/math/rand-8         	 8213564	       138 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-16/crypto/rand-8       	 9739500	       123 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-16/randbp-8            	  979442	      1329 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-64/math/rand-8         	 5289319	       227 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-64/crypto/rand-8       	 6050103	       197 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-64/randbp-8            	  911760	      1301 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-256/math/rand-8        	 4223463	       274 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-256/crypto/rand-8      	 2263252	       534 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-256/randbp-8           	  940459	      1333 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-512/math/rand-8        	 2455426	       481 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-512/crypto/rand-8      	 1000000	      1008 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-512/randbp-8           	  885555	      1445 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-1024/math/rand-8       	 1275535	       925 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-1024/crypto/rand-8     	  636202	      1980 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-1024/randbp-8          	  800511	      1630 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-4096/math/rand-8       	  310982	      3765 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-4096/crypto/rand-8     	  159490	      7538 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-4096/randbp-8          	  511124	      2319 ns/op	       0 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-1048576/math/rand-8    	    1341	    860809 ns/op	    6255 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-1048576/crypto/rand-8  	     838	   1349225 ns/op	   10016 B/op	       0 allocs/op
//	BenchmarkRand/Read/size-1048576/randbp-8       	    5330	    238657 ns/op	    1582 B/op	       0 allocs/op
//	PASS
//	ok  	github.com/reddit/baseplate.go/randbp	29.982s
//
// Regardless performance, it's never suitable for security purpose,
// and you should always use crypto/rand for that instead.
func (r Rand) Read(p []byte) (int, error) {
	reader := readerPool.Get().(*rand.Rand)
	defer readerPool.Put(reader)

	reader.Seed(int64(r.Uint64()))
	return reader.Read(p)
}

// New initializes a thread-safe, properly seeded Rand.
func New(seed int64) Rand {
	return Rand{
		Rand: rand.New(NewLockedSource64(rand.NewSource(seed))),
	}
}
