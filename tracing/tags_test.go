package tracing

import (
	"reflect"
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
