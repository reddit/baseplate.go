// Package redisbp provides Baseplate integrations for go-redis.
//
// See https://pkg.go.dev/github.com/redis/go-redis/v9 for documentation for
// go-redis.
//
// It's recommended to be used in "use Redis as a DB" scenarios as it provides
// Wait function to achieve guaranteed write consistency.
// For other use cases redispipebp is preferred.
package redisbp
