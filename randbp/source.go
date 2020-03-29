package randbp

import (
	"math/rand"
	"sync"
)

var _ rand.Source64 = (*LockedSource64)(nil)

type source64 interface {
	Uint64() uint64
}

// LockedSource64 is a thread-safe implementation of rand.Source64.
//
// NOTE: When using *LockedSource64 to create rand.Rand,
// its Read function will have a much worse performance comparing to rand's
// global rander or rand.Rand created with non-thread-safe source.
type LockedSource64 struct {
	src  rand.Source
	s64  source64
	lock sync.Mutex
}

// NewLockedSource64 creates a *LockedSource64 from the given src.
func NewLockedSource64(src rand.Source) *LockedSource64 {
	ls := &LockedSource64{
		src: src,
	}
	ls.setS64()
	return ls
}

func (ls *LockedSource64) setS64() {
	s64, ok := ls.src.(source64)
	if !ok {
		// *rand.Rand also implements source64, and it calls Int63 twice to generate
		// uint64 when the source isn't rand.Source64. Take advantage of that.
		s64 = rand.New(ls.src)
	}
	ls.s64 = s64
}

// Int63 implements rand.Source64.
//
// It calls underlying source's Int63 with lock.
func (ls *LockedSource64) Int63() (n int64) {
	ls.lock.Lock()
	n = ls.src.Int63()
	ls.lock.Unlock()
	return
}

// Uint64 implements rand.Source64.
//
// If the underlying source implements rand.Source64,
// it calls its Uint64 with lock.
// Otherwise, it calls its Int64 twice with lock.
func (ls *LockedSource64) Uint64() (n uint64) {
	ls.lock.Lock()
	n = ls.s64.Uint64()
	ls.lock.Unlock()
	return
}

// Seed implements rand.Source64.
//
// It calls underlying source's Seed with lock.
func (ls *LockedSource64) Seed(seed int64) {
	ls.lock.Lock()
	ls.src.Seed(seed)
	ls.setS64()
	ls.lock.Unlock()
}
