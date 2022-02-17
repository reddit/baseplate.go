package redisx

import (
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/joomcode/redispipe/redis"
)

var (
	intArrayCommands = map[string]struct{}{
		"BITFIELD":      {},
		"SCRIPT EXISTS": {},
	}

	keyValueArrayCommands = map[string]struct{}{
		"HMGET": {},
		"MGET":  {},
	}

	bytesArrayCommands = map[string]struct{}{
		"BLPOP":            {},
		"BRPOP":            {},
		"GEOHASH":          {},
		"HKEYS":            {},
		"HVALS":            {},
		"KEYS":             {},
		"LRANGE":           {},
		"SDIFF":            {},
		"SINTER":           {},
		"SMEMBERS":         {},
		"SPOPN":            {},
		"SRANDMEMBERN":     {},
		"SUNION":           {},
		"XCLAIMJUSTID":     {},
		"ZRANGE":           {},
		"ZRANGEBYLEX":      {},
		"ZRANGEBYSCORE":    {},
		"ZREVRANGE":        {},
		"ZREVRANGEBYLEX":   {},
		"ZREVRANGEBYSCORE": {},
	}

	bytesArrayT     = reflect.TypeOf([][]byte{})
	byteArrayT      = reflect.TypeOf([]byte{})
	intArrayT       = reflect.TypeOf([]int64{})
	interfaceArrayT = reflect.TypeOf([]interface{}{})
	stringPtrT      = reflect.TypeOf((*string)(nil))
	intPtrT         = reflect.TypeOf((*int64)(nil))
)

// Req is a convenience function for creating new Request objects.
func Req(v interface{}, cmd string, args ...interface{}) Request {
	return Request{redis.Req(strings.ToUpper(cmd), args...), v}
}

// Request wraps the Request object from redispipe and also holds the response
// input passed in by the caller that we will use to set the response from Redis.
type Request struct {
	redis.Request

	V interface{}
}

// setValue uses reflection to set the value of r.V to res.
func (r Request) setValue(res interface{}) error {
	if r.V == nil || res == nil {
		return nil
	}

	rv := reflect.ValueOf(r.V)
	// We don't bother checking rv.IsValid() because:
	// 1. The reason that would be false is if r.V is nil, which is explicitly
	//    checked above and used as  a signal to ignore the result.
	// 2. Kind() does not panic, it just returns invalid which would fail this
	//    check anyways.
	if rv.Kind() != reflect.Ptr {
		return &InvalidInputError{
			Message: "request.V must be a pointer, got " + rv.Kind().String(),
		}
	}

	e := rv.Elem()
	// We can skip checking e.IsValid since CanSet will return false rather than
	// panic if e is invalid.
	if !e.CanSet() {
		return &InvalidInputError{
			Message: "the value pointed to by request.V cannot be set " + e.String(),
		}
	}

	if !isSupportedInput(e) {
		return &InvalidInputError{
			Message: e.Type().Name() + " is not a supported response input type to redisx",
		}
	}

	rRes := reflect.ValueOf(res)
	// We can skip checking rRes.IsValid since we explicitly handle rRes being nil
	// which is what would cause this to return false.

	if e.Type().AssignableTo(rRes.Type()) {
		e.Set(rRes)
		return nil
	} else if e.Kind() == reflect.Struct {
		return r.setStructValue(e, rRes)
	} else if e.Type() == bytesArrayT {
		return r.setByteArrayValue(e, rRes)
	} else if e.Type() == intArrayT {
		return r.setIntArrayValue(e, rRes)
	} else if rRes.Type() == byteArrayT {
		return r.convertAndSetByteSlice(e, rRes)
	}
	return &ResponseInputTypeError{
		Cmd:               r.Cmd,
		ResponseInputType: e.Type(),
	}
}

// setByteArrayValue converts src to a [][]byte and sets src to the converted dst.
// Assumes you have already checked that dst is a [][]byte and will panic if it
// is not.
func (r Request) setByteArrayValue(dst reflect.Value, src reflect.Value) error {
	if _, ok := bytesArrayCommands[r.Cmd]; !ok {
		return &ResponseInputTypeError{
			Cmd:               r.Cmd,
			ResponseInputType: dst.Type(),
		}
	}
	if src.Type() != interfaceArrayT {
		return &UnexpectedResponseError{
			Message: "redispipe returned unexpected response type " + src.String() + ", expected []interface{}",
		}
	}

	response, _ := src.Interface().([]interface{})
	val := make([][]byte, 0, len(response))
	for _, r := range response {
		b, _ := r.([]byte)
		val = append(val, b)
	}

	// This has the potential to panic but really can't as we have already checked
	// that dst is a "[][]byte" and val is also a "[][]byte"
	dst.Set(reflect.ValueOf(val))
	return nil
}

// setIntArrayValue converts src to an []int64 and sets scr to the converted dst.
// assumes you have already checked that dst is an []int64 and will panic if it
// is not.
func (r Request) setIntArrayValue(dst reflect.Value, src reflect.Value) error {
	if _, ok := intArrayCommands[r.Cmd]; !ok {
		return &ResponseInputTypeError{
			Cmd:               r.Cmd,
			ResponseInputType: dst.Type(),
		}
	}
	if src.Type() != interfaceArrayT {
		return &UnexpectedResponseError{
			Message: "redispipe returned unexpected response type" + src.String() + ", expected []interface{}",
		}
	}

	response, _ := src.Interface().([]interface{})
	val := make([]int64, 0, len(response))
	for _, r := range response {
		i, _ := r.(int64)
		val = append(val, i)
	}

	// This has the potential to panic but really can't as we have already checked
	// that dst is a "[]int64" and val is also a "[]int64"
	dst.Set(reflect.ValueOf(val))
	return nil
}

// setStructValue sets values from src into fields on the struct referenced by
// dst.
func (r Request) setStructValue(dst reflect.Value, src reflect.Value) error {
	if _, ok := keyValueArrayCommands[r.Cmd]; !ok {
		return &ResponseInputTypeError{
			Cmd:               r.Cmd,
			ResponseInputType: dst.Type(),
		}
	}
	if src.Type() != interfaceArrayT {
		return &UnexpectedResponseError{
			Message: "redispipe returned unexpected response type" + src.String() + ", expected []interface{}",
		}
	}

	var args []interface{}

	// The "keys" we want for HMGET are the keys in the hash, the first key is
	// the hash key.
	if r.Cmd == "HMGET" {
		args = r.Args[1:]
	} else if r.Cmd == "MGET" {
		args = r.Args
	} else {
		return &ResponseInputTypeError{
			Cmd:               r.Cmd,
			ResponseInputType: dst.Type(),
		}
	}
	keys := make([]string, 0, len(args))
	for _, a := range args {
		keys = append(keys, a.(string))
	}

	response := src.Interface().([]interface{})

	if len(keys) != len(response) {
		return &UnexpectedResponseError{
			Message: "command " + r.Cmd + " does not have the same number of respones as keys passed into it",
		}
	}
	fields := cachedStructFields(dst.Type())
	for i, key := range keys {
		v := response[i]
		// Discard nil values
		if v == nil {
			continue
		}

		f, ok := fields[key]
		// Skip keys that are not set as fields
		if !ok {
			continue
		}
		field := dst.Field(f.index)
		value := reflect.ValueOf(v)
		if field.Type().AssignableTo(value.Type()) {
			field.Set(value)
		} else if value.Type() == byteArrayT {
			if err := r.convertAndSetByteSlice(field, value); err != nil {
				return err
			}
		} else {
			return &UnexpectedResponseError{
				Message: "command " + r.Cmd + " returned a value of an unexpected type " + value.String(),
			}
		}
	}
	return nil
}

func (r Request) convertAndSetByteSlice(dst reflect.Value, src reflect.Value) error {
	asBytes, _ := src.Interface().([]byte)
	asStr := string(asBytes)
	switch dst.Kind() {
	case reflect.Int64:
		asInt, err := strconv.ParseInt(asStr, 10, 64)
		if err != nil {
			return &InvalidInputError{
				Message: "could not parse input " + asStr + " into an int64",
			}
		}
		dst.Set(reflect.ValueOf(asInt))
	case reflect.String:
		dst.Set(reflect.ValueOf(asStr))
	default:
		switch dst.Type() {
		case stringPtrT:
			dst.Set(reflect.ValueOf(&asStr))
		case intPtrT:
			asInt, err := strconv.ParseInt(asStr, 10, 64)
			if err != nil {
				return &InvalidInputError{
					Message: "could not parse input " + asStr + " into an int64",
				}
			}
			dst.Set(reflect.ValueOf(&asInt))
		default:
			return &ResponseInputTypeError{
				Cmd:               r.Cmd,
				ResponseInputType: dst.Type(),
			}
		}
	}
	return nil
}

type structField struct {
	name  string
	index int
}

// a similar pattern of caching the struct fields is used by:
// encoding/json
// github.com/mediocregopher/radix
//
// this is done to avoid re-parsing the structure of a struct each time you
// decode a redispipe response into it.
var structFieldsCache sync.Map // map[reflect.Type]map[string]structField

// getStructFields, but uses the structFieldsCache
func cachedStructFields(t reflect.Type) map[string]structField {
	if sf, ok := structFieldsCache.Load(t); ok {
		return sf.(map[string]structField)
	}
	sf, _ := structFieldsCache.LoadOrStore(t, getStructFields(t))
	return sf.(map[string]structField)
}

// getStructFields extracts a mapping of "reference" (either field name or tag)
// to structField for the given reflect.Type.
// use cachedStructFields rather than this directly to avoid duplicating work.
func getStructFields(t reflect.Type) map[string]structField {
	structFields := map[string]structField{}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	numFields := t.NumField()
	for i := 0; i < numFields; i++ {
		f := t.Field(i)
		// Skip embedded (f.Anonymous) or unexported (empty f.PkgPath) fields
		if f.Anonymous || f.PkgPath != "" {
			continue
		}
		sf := structField{
			name:  f.Name,
			index: i,
		}
		key := strings.ToLower(sf.name)
		if tag := f.Tag.Get("redisx"); tag != "" {
			// Skip fields tagged with "-"
			if tag == "-" {
				continue
			}
			key = tag
		}
		structFields[key] = sf
	}
	return structFields
}

func isSupportedInput(e reflect.Value) bool {
	return e.Kind() == reflect.Int64 ||
		e.Kind() == reflect.String ||
		e.Kind() == reflect.Struct ||
		e.Type() == byteArrayT ||
		e.Type() == bytesArrayT ||
		e.Type() == intArrayT ||
		e.Type() == interfaceArrayT ||
		e.Type() == stringPtrT ||
		e.Type() == intPtrT
}
