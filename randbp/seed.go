package randbp

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

// GetSeed returns a seed for pseudo-random generator.
//
// It tries to use crypto/rand to read an int64,
// and fallback to use current time if that fails for whatever reason.
func GetSeed() int64 {
	buf := make([]byte, 8)
	_, err := rand.Read(buf)
	if err != nil {
		return time.Now().UnixNano()
	}
	return int64(binary.BigEndian.Uint64(buf))
}
