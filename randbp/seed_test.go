package randbp

import (
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"testing"
)

func TestGetSeed(t *testing.T) {
	const maxBytes = 8

	oldCryptoReader := cryptoReader
	oldLogOutput := getSeedLogOutput
	defer func() {
		cryptoReader = oldCryptoReader
		getSeedLogOutput = oldLogOutput
	}()

	getSeedLogOutput = ioutil.Discard

	readerGenerator := func(t *testing.T, n int) func(p []byte) (int, error) {
		var err error
		if n < maxBytes {
			err = errors.New("error")
		}

		return func(p []byte) (int, error) {
			t.Helper()

			ret := n
			if n < len(p) {
				p = p[:n]
			} else {
				ret = len(p)
			}
			if _, readErr := R.Read(p); readErr != nil {
				t.Error(readErr)
			}
			return ret, err
		}
	}

	const (
		// Number of concurrent GetSeed calls
		n = 1000

		// Don't use 100% unique as the target as the randomness nature of this
		// test, but the randomness shouldn't cause it to be fewer than 90% unique
		// unless there's a bug in the implementation.
		target = int(n * 0.9)
	)
	for i := 0; i <= maxBytes; i++ {
		t.Run(fmt.Sprintf("read-%d-bytes", i), func(t *testing.T) {
			cryptoReader = readerGenerator(t, i)
			set := make(map[int64]bool)
			var lock sync.Mutex
			var wg sync.WaitGroup
			wg.Add(n)
			for j := 0; j < n; j++ {
				go func() {
					defer wg.Done()
					seed := GetSeed()
					lock.Lock()
					defer lock.Unlock()
					set[seed] = true
				}()
			}
			wg.Wait()

			if t.Failed() {
				// In case that readerGenerator failed to generate required bytes,
				// fail early.
				t.FailNow()
			}

			size := len(set)
			t.Logf("%d unique seeds among %d tries", size, n)
			// This will fail for i == 0 and probably also i == 1 when someone
			// accidentally change bitwise xor to bitwise and in the implementation.
			if size < target {
				t.Errorf(
					"Too few unique seeds returned: %v",
					set,
				)
			}
		})
	}
}
