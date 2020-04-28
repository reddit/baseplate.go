package signing_test

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"

	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/signing"
)

// A random time.Duration that's between 10 seconds and 1 hour 10 seconds.
type randomDuration time.Duration

func (randomDuration) Generate(r *rand.Rand, _ int) reflect.Value {
	return reflect.ValueOf(randomDuration(
		r.Int63n(int64(time.Hour)) + int64(time.Second*10),
	))
}

// A random []byte with a length in [1, 4096].
type randomByteSlice []byte

func (randomByteSlice) Generate(r *rand.Rand, _ int) reflect.Value {
	n := r.Intn(4096) + 1
	slice := make([]byte, n)
	for i := range slice {
		slice[i] = byte(r.Intn(256))
	}
	return reflect.ValueOf(randomByteSlice(slice))
}

var (
	_ quick.Generator = randomDuration(0)
	_ quick.Generator = randomByteSlice(nil)
)

func TestV1Quick(t *testing.T) {
	f := func(msg, key randomByteSlice, expiresIn randomDuration) bool {
		secret := secrets.VersionedSecret{Current: secrets.Secret(key)}
		args := signing.SignArgs{
			Message:   []byte(msg),
			Secret:    secret,
			ExpiresIn: time.Duration(expiresIn),
		}
		signature, err := signing.V1.Sign(args)
		if err != nil {
			t.Error(err)
			return false
		}
		err = signing.V1.Verify([]byte(msg), signature, secret)
		if err != nil {
			t.Error(err)
			return false
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}
