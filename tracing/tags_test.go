package tracing

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func compareStringListsIgnoreOrder(t *testing.T, a, b []string) {
	t.Helper()

	if len(a) != len(b) {
		t.Errorf("Length mismatch: %#v vs. %#v", a, b)
		return
	}

	ma := make(map[string]struct{}, len(a))
	mb := make(map[string]struct{}, len(b))
	var value struct{}
	for _, i := range a {
		ma[i] = value
	}
	for _, i := range b {
		mb[i] = value
	}
	if !reflect.DeepEqual(ma, mb) {
		t.Errorf("%#v != %#v", a, b)
	}
}

func TestGenerateAllowList(t *testing.T) {
	for _, c := range []struct {
		label  string
		input  []string
		output []string
	}{
		{
			label:  "empty",
			output: alwaysIncludeAllowList,
		},
		{
			label: "one",
			input: []string{"foo"},
			output: []string{
				"foo",
				TagKeyClient,
				TagKeyEndpoint,
			},
		},
		{
			label: "dup",
			input: []string{
				"foo",
				TagKeyClient,
				"foo",
			},
			output: []string{
				"foo",
				TagKeyClient,
				TagKeyEndpoint,
			},
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			actual := generateAllowList(c.input)
			compareStringListsIgnoreOrder(t, c.output, actual)
		})
	}
}

func TestSetMetricsTagsAllowList(t *testing.T) {
	t.Run("mutate-element", func(t *testing.T) {
		const (
			before = "before"
			after  = "after"
		)
		orig := []string{before, TagKeyClient, TagKeyEndpoint}
		SetMetricsTagsAllowList(orig)
		loaded := *tagsAllowList.Load()
		orig[0] = after

		compareStringListsIgnoreOrder(t, loaded, []string{before, TagKeyClient, TagKeyEndpoint})
	})

	t.Run("compare-pointer", func(t *testing.T) {
		// Make sure that SetMetricsTagsAllowList always stores a copy of the original
		// slice passed in.
		for _, c := range [][]string{} {
			orig := c
			t.Run(strings.Join(orig, ":"), func(t *testing.T) {
				SetMetricsTagsAllowList(orig)
				loaded := *tagsAllowList.Load()
				if fmt.Sprintf("%p", orig) == fmt.Sprintf("%p", loaded) {
					t.Errorf("Loaded the same slice: %p %+v == %p %+v", orig, orig, loaded, loaded)
				}
			})
		}
	})
}
