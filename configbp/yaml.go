package configbp

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v2"
)

// Int64String is an int64 type that can be yaml deserialized from strings.
//
// It's useful when the yaml config goes through helm,
// which could cause precision loss on large int64 numbers:
// https://github.com/helm/helm/issues/11045
type Int64String int64

var (
	_ yaml.Unmarshaler = (*Int64String)(nil)
)

// UnmarshalYAML implements yaml.Unmarshaler.
func (i *Int64String) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	i64, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("cannot parse %q as int64: %v", s, err)
	}
	*i = Int64String(i64)
	return nil
}
