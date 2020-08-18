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

// RandomStringArgs defines the args used by GenerateRandomString.
type RandomStringArgs struct {
	// Required. If MaxLength <= MinLength it will cause panic.
	MaxLength int

	// Optional. Default is 0, which means it could generate empty strings.
	// If MinLength < 0 or MinLength >= MaxLength it will cause panic.
	MinLength int

	// Optional. If nil randbp.R will be used instead.
	R *rand.Rand

	// Optional. If empty []rune(randbp.Base64Runes) will be used instead.
	Runes []rune
}

// The common interface between *math/rand.Rand and randbp.Rand used in
// GenerateRandomString.
type intner interface {
	Intn(n int) int
}

// GenerateRandomString generates a random string with length
// [MinLength, MaxLength), and all characters limited to Runes.
//
// It could be used to help implement testing/quick.Generator interface.
func GenerateRandomString(args RandomStringArgs) string {
	var r intner = args.R
	if r == nil {
		r = R
	}
	runes := args.Runes
	if len(runes) == 0 {
		runes = []rune(Base64Runes)
	}
	n := r.Intn(args.MaxLength-args.MinLength) + args.MinLength
	ret := make([]rune, n)
	for i := range ret {
		ret[i] = runes[r.Intn(len(runes))]
	}
	return string(ret)
}
