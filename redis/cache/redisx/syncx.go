package redisx

import (
	"context"
	"errors"

	"github.com/joomcode/redispipe/redis"
)

// Syncx is a wrapper around a Sync that provides an API that uses reflection to
// allow you to set the response from your redis command into a typed variable
// without having to manually cast it from "interface{}".
//
// Methods no longer return an interface{}, instead they return only errors and
// you pass in a pointer to the value that you want to have the response put
// into.
//
// The values that you can use as the "response" input vary depending on the
// command.  You may also pass in 'nil' if you want to ignore any non-error
// results.
// In addition to errors, Redis commands return one of 4 types of responses:
//
//  1. Simple Strings: "*string" is the only type you can use as the response
//     input.
//  2. Integers: "*int64" is the only type you can use as the response input.
//  3. Bulk strings: "*[]byte", the type returned by redispipe for these commands,
//     is supported as a response input.  In additon, you may use a "string" or
//     "int64" and we will attempt to convert the "[]byte" response to that value.
//     Note that this is done by converting it to a "string" first and, in the
//     case of "int64", parsing it into an integer. You can also use "*string"
//     or "*int64", to be able to represent null value.
//  4. Arrays: Arrays are the most flexible type returned by Redis as an array
//     can contain elements of any of the types above. "[]interface{}" is the
//     type that redispipe returns on these commands and is supported as a
//     response input.  Certain commands support more specific inputs though,
//     specifically "*[]int64" or "*[][]byte" since they always return arrays
//     that only contain a specific type.
//
// Array Commands that support "*[][]byte":
//
//   - BLPOP
//   - BRPOP
//   - GEOHASH
//   - HKEYS
//   - HVALS
//   - KEYS
//   - LRANGE
//   - SDIFF
//   - SINTER
//   - SMEMBERS
//   - SPOPN
//   - SRANDMEMBERN
//   - SUNION
//   - XCLAIMJUSTID
//   - ZPOPMAX
//   - ZPOPMIN
//   - ZRANGE
//   - ZRANGEBYLEX
//   - ZRANGEBYSCORE
//   - ZREVRANGE
//   - ZREVRANGEBYLEX
//   - ZREVRANGEBYSCORE
//
// Array Commands that support "*[]int64":
//
//   - BITFIELD
//   - SCRIPT EXISTS
//
// In addition, some commands, specifically key/value commands, support
// Struct Scanning where you can pass in a pointer to an arbitrary struct and
// it will put the response values into the struct by mapping key values to
// field names.  It also supports specifying keys using the "redisx" tag and
// fields with the tag "-" will be ignored. When mapping to field names, the
// mapping is case insensitive while tags are case sensitive.  Fields in the
// struct that map to keys in the response must be one of "[]byte", "int64",
// or "string", depending on what you expect to be returned by that key, using
// other types will result in returning an error.
//
// Array Commands that support Struct Scanning:
//
//   - HMGET
//   - MGET
type Syncx struct {
	Sync Sync
}

// Do is a convenience wrapper for s.Send.  It does not use s.Sync.Do.
func (s Syncx) Do(ctx context.Context, v interface{}, cmd string, args ...interface{}) error {
	return s.Send(ctx, Req(v, cmd, args...))
}

// Send sends a single request to redis.
func (s Syncx) Send(ctx context.Context, r Request) error {
	res := s.Sync.Send(ctx, r.Request)
	if err := redis.AsError(res); err != nil {
		return err
	}
	return r.setValue(res)
}

// SendMany sends multiple requests to redis. It returns a slice of errors that
// is the same length as the number of requests passed to SendMany. The error at
// an index in the response is the error for the request at the same index. If
// there was no error for that individual request, then the entry in the response
// will be nil.
// These requests are not sent as a transaction, use SendTransaction if you wish
// to do that.
func (s Syncx) SendMany(ctx context.Context, reqs ...Request) []error {
	errs := make([]error, len(reqs))
	results := s.Sync.SendMany(ctx, toRedispipeRequests(reqs))
	for i, res := range results {
		if err := redis.AsError(res); err != nil {
			errs[i] = err
		} else {
			errs[i] = reqs[i].setValue(res)
		}
	}
	return errs
}

// SendTransaction sends multiple requests to redis in a single transaction. It
// returns a single error since a transaction either succeeds entirely or it fails.
// The response may be a errorsbp.Batch.
func (s Syncx) SendTransaction(ctx context.Context, reqs ...Request) error {
	results, err := s.Sync.SendTransaction(ctx, toRedispipeRequests(reqs))
	if err != nil {
		return err
	}
	errs := make([]error, 0, len(results))
	for i, res := range results {
		errs = append(errs, reqs[i].setValue(res))
	}
	return errors.Join(errs...)
}

// Scanner returns a ScanIterator from the underlying Sync.
func (s Syncx) Scanner(ctx context.Context, opts redis.ScanOpts) ScanIterator {
	return s.Sync.Scanner(ctx, opts)
}

func toRedispipeRequests(reqs []Request) []redis.Request {
	r := make([]redis.Request, 0, len(reqs))
	for _, req := range reqs {
		r = append(r, req.Request)
	}
	return r
}
