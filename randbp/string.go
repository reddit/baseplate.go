package randbp

import (
	"math/rand"
)

// Base64Runes are all the runes allowed in standard and url safe base64
// encodings.
//
// This is a common, safe to use set of runes to be used with
// GenerateRandomString.
const Base64Runes = `ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_+/=`

// GenerateRandomString generates a random string with length [0, maxLength),
// and all characters limited to runes.
//
// It could be used to help implement testing/quick.Generator interface.
func GenerateRandomString(r *rand.Rand, maxLength int, runes []rune) string {
	n := r.Intn(maxLength)
	ret := make([]rune, n)
	for i := range ret {
		ret[i] = runes[r.Intn(len(runes))]
	}
	return string(ret)
}
