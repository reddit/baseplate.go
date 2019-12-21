package set_test

import (
	"testing"

	"github.com/reddit/baseplate.go/set"
)

func TestStringSet(t *testing.T) {
	const (
		key1 = "key1"
		key2 = "key2"
	)

	var s set.String

	t.Run(
		"Zero",
		func(t *testing.T) {
			if s.Contains(key1) {
				t.Errorf("%v should not contain item %q", s, key1)
			}
			if s.Contains(key2) {
				t.Errorf("%v should not contain item %q", s, key2)
			}
			expectedStr := "{}"
			actualStr := s.String()
			if actualStr != expectedStr {
				t.Errorf("s.String() expected to be %q, got %q", expectedStr, actualStr)
			}
		},
	)

	t.Run(
		"New",
		func(t *testing.T) {
			s = make(set.String)
			if s.Contains(key1) {
				t.Errorf("%v should not contain item %q", s, key1)
			}
			if s.Contains(key2) {
				t.Errorf("%v should not contain item %q", s, key2)
			}
			expectedStr := "{}"
			actualStr := s.String()
			if actualStr != expectedStr {
				t.Errorf("s.String() expected to be %q, got %q", expectedStr, actualStr)
			}
		},
	)

	t.Run(
		"OneItem",
		func(t *testing.T) {
			s.Add(key1)
			if !s.Contains(key1) {
				t.Errorf("%v should contain item %q", s, key1)
			}
			if s.Contains(key2) {
				t.Errorf("%v should not contain item %q", s, key2)
			}
			expectedStr := `{"key1"}`
			actualStr := s.String()
			if actualStr != expectedStr {
				t.Errorf("s.String() expected to be %q, got %q", expectedStr, actualStr)
			}
		},
	)

	t.Run(
		"DupItem",
		func(t *testing.T) {
			s.Add(key1)
			if !s.Contains(key1) {
				t.Errorf("%v should contain item %q", s, key1)
			}
			if s.Contains(key2) {
				t.Errorf("%v should not contain item %q", s, key2)
			}
			expectedStr := `{"key1"}`
			actualStr := s.String()
			if actualStr != expectedStr {
				t.Errorf("s.String() expected to be %q, got %q", expectedStr, actualStr)
			}
		},
	)

	t.Run(
		"SecondItem",
		func(t *testing.T) {
			s.Add(key2)
			if !s.Contains(key1) {
				t.Errorf("%v should contain item %q", s, key1)
			}
			if !s.Contains(key2) {
				t.Errorf("%v should contain item %q", s, key2)
			}
			// map iteration has no guaranteed order so we enumerate both cases.
			expectedStr1 := `{"key2", "key1"}`
			expectedStr2 := `{"key1", "key2"}`
			actualStr := s.String()
			if actualStr != expectedStr1 && actualStr != expectedStr2 {
				t.Errorf(
					"s.String() expected to be either %q or %q, got %q",
					expectedStr1,
					expectedStr2,
					actualStr,
				)
			}
		},
	)

	t.Run(
		"Remove",
		func(t *testing.T) {
			s.Remove(key2)
			if !s.Contains(key1) {
				t.Errorf("%v should contain item %q", s, key1)
			}
			if s.Contains(key2) {
				t.Errorf("%v should not contain item %q", s, key2)
			}
			expectedStr := `{"key1"}`
			actualStr := s.String()
			if actualStr != expectedStr {
				t.Errorf("s.String() expected to be %q, got %q", expectedStr, actualStr)
			}
		},
	)

	t.Run(
		"RemoveAgain",
		func(t *testing.T) {
			s.Remove(key2)
			if !s.Contains(key1) {
				t.Errorf("%v should contain item %q", s, key1)
			}
			if s.Contains(key2) {
				t.Errorf("%v should not contain item %q", s, key2)
			}
			expectedStr := `{"key1"}`
			actualStr := s.String()
			if actualStr != expectedStr {
				t.Errorf("s.String() expected to be %q, got %q", expectedStr, actualStr)
			}
		},
	)

	t.Run(
		"SecondItemAgain",
		func(t *testing.T) {
			s.Add(key2)
			if !s.Contains(key1) {
				t.Errorf("%v should contain item %q", s, key1)
			}
			if !s.Contains(key2) {
				t.Errorf("%v should contain item %q", s, key2)
			}
			// map iteration has no guaranteed order so we enumerate both cases.
			expectedStr1 := `{"key2", "key1"}`
			expectedStr2 := `{"key1", "key2"}`
			actualStr := s.String()
			if actualStr != expectedStr1 && actualStr != expectedStr2 {
				t.Errorf(
					"s.String() expected to be either %q or %q, got %q",
					expectedStr1,
					expectedStr2,
					actualStr,
				)
			}
		},
	)

	t.Run(
		"ToSlice",
		func(t *testing.T) {
			compareSlices := func(s1, s2 []string) bool {
				if len(s1) != len(s2) {
					return false
				}
				for i := range s1 {
					if s1[i] != s2[i] {
						return false
					}
				}
				return true
			}

			// ToSlice has no guaranteed order so we enumerate both cases.
			expectedSlice1 := []string{key1, key2}
			expectedSlice2 := []string{key2, key1}
			slice := s.ToSlice()
			if !compareSlices(expectedSlice1, slice) && !compareSlices(expectedSlice2, slice) {
				t.Errorf(
					"s.ToSlice() expected to be either %+v or %+v, got %+v",
					expectedSlice1,
					expectedSlice2,
					slice,
				)
			}
		},
	)

	t.Run(
		"Equals",
		func(t *testing.T) {
			other := set.StringSliceToSet([]string{key1, key2})
			if !s.Equals(other) {
				t.Errorf("%v.Equals(%v) should be true", s, other)
			}
			if !other.Equals(s) {
				t.Errorf("%v.Equals(%v) should be true", other, s)
			}

			other = set.StringSliceToSet([]string{key1})
			if s.Equals(other) {
				t.Errorf("%v.Equals(%v) should be false", s, other)
			}
			if other.Equals(s) {
				t.Errorf("%v.Equals(%v) should be false", other, s)
			}

			other = make(set.String)
			if s.Equals(other) {
				t.Errorf("%v.Equals(%v) should be false", s, other)
			}
			if other.Equals(s) {
				t.Errorf("%v.Equals(%v) should be false", other, s)
			}

			empty1 := make(set.String)
			empty2 := set.StringSliceToSet(nil)
			if !empty1.Equals(empty2) {
				t.Errorf("%v.Equals(%v) should be true", empty1, empty2)
			}
			if !empty2.Equals(empty1) {
				t.Errorf("%v.Equals(%v) should be true", empty2, empty1)
			}
		},
	)
}
