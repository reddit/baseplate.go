package randbp

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

// Define as variables so they can be mocked in unit tests.
var (
	cryptoReader               = rand.Read
	getSeedLogOutput io.Writer = os.Stderr
)

// GetSeed returns an int64 seed for pseudo-random generator.
//
// It tries to use crypto/rand to read an int64,
// and if an error happens, it logs the error to stderr,
// then fallback to use current time bitwise xor the part it read from
// crypto/rand instead.
//
// The bitwise xor part in the fallback is in order to try to prevent two
// processes starting at the same time getting the same seed from happening.
func GetSeed() int64 {
	buf := make([]byte, 8)
	n, err := cryptoReader(buf)
	seed := int64(binary.LittleEndian.Uint64(buf))
	if err != nil {
		fmt.Fprintf(
			getSeedLogOutput,
			"randbp.GetSeed: only read %d bytes from crypto/rand: %v\n",
			n,
			err,
		)
		return seed ^ time.Now().UnixNano()
	}
	return seed
}
