package randbp

import (
	randv1 "math/rand"
	"math/rand/v2"
)

// Base64Runes are all the runes allowed in standard and url safe base64
// encodings.
//
// This is a common, safe to use set of runes to be used with
// GenerateRandomString.
const Base64Runes = `ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_+/=`

// RandomStringArgs defines the args used by GenerateRandomString.
type RandomStringArgs struct {
	// Required. If MaxLength <= MinLength it will cause panic.
	MaxLength int

	// Optional. Default is 0, which means it could generate empty strings.
	// If MinLength < 0 or MinLength >= MaxLength it will cause panic.
	MinLength int

	// Optional. If empty []rune(randbp.Base64Runes) will be used instead.
	Runes []rune

	// Deprecated: This is no longer used. We always use math/rand/v2 global
	// PRNG.
	R *randv1.Rand
}

// GenerateRandomString generates a random string with length
// [MinLength, MaxLength), and all characters limited to Runes.
//
// It could be used to help implement testing/quick.Generator interface.
func GenerateRandomString(args RandomStringArgs) string {
	runes := args.Runes
	if len(runes) == 0 {
		runes = []rune(Base64Runes)
	}
	n := rand.IntN(args.MaxLength-args.MinLength) + args.MinLength
	ret := make([]rune, n)
	for i := range ret {
		ret[i] = runes[rand.IntN(len(runes))]
	}
	return string(ret)
}
