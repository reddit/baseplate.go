package set

import (
	"fmt"
	"strings"
)

// String defines the string set.
//
// You should initialize it with make(String),
// or use StringSliceToSet to convert an existing slice.
type String map[string]Value

// StringSliceToSet creates a new string set from the existing slice.
func StringSliceToSet(slice []string) String {
	set := make(String, len(slice))
	for _, s := range slice {
		set.Add(s)
	}
	return set
}

// Add adds an item to the set.
func (s String) Add(item string) {
	s[item] = DummyValue
}

// Remove removes an item from the set.
//
// NOTE: Due to a go runtime bug[1],
// Remove works functionally but does not free the memory held by removed
// item(s).
//
// [1] https://github.com/golang/go/issues/20135
func (s String) Remove(item string) {
	delete(s, item)
}

// Contains returns true if item is in the set.
func (s String) Contains(item string) bool {
	_, ok := s[item]
	return ok
}

// ToSlice converts the set into a string slice.
//
// There's no guaranteed order of the slice to be returned.
func (s String) ToSlice() []string {
	slice := make([]string, 0, len(s))
	for str := range s {
		slice = append(slice, str)
	}
	return slice
}

// Equals returns true if this string set equals to the other string set.
func (s String) Equals(other String) bool {
	if len(s) != len(other) {
		return false
	}
	for str := range s {
		if !other.Contains(str) {
			return false
		}
	}
	return true
}

func (s String) String() string {
	var sb strings.Builder
	sb.WriteString("{")

	first := true
	for item := range s {
		if first {
			first = false
		} else {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%q", item))
	}

	sb.WriteString("}")
	return sb.String()
}
