// Package randbp provides some random generator related features:
//
// 1. Proper seed the global math/rand generator.
// 2. Helper functions for common use cases.
//
// For users using only 1 (e.g. you use the global math/rand generator for
// things other than the helper functions provided by this package),
// you should blank import this package in your main package, e.g.
//
//     import (
//       _ "github.com/reddit/baseplate.go/randbp"
//     )
package randbp

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}
