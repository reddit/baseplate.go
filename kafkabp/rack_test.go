package kafkabp_test

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/kafkabp"
)

func TestRackIDFuncUnmarshalText(t *testing.T) {
	getActualFuncName := func(r kafkabp.RackIDFunc) string {
		// This returns something like:
		// "github.com/reddit/baseplate.go/kafkabp.AWSAvailabilityZoneRackID"
		return runtime.FuncForPC(reflect.ValueOf(r).Pointer()).Name()
	}

	for _, c := range []struct {
		text     string
		expected string
	}{
		{
			text:     "aws",
			expected: "AWSAvailabilityZoneRackID",
		},
		{
			text:     "http://www.google.com",
			expected: "SimpleHTTPRackID",
		},
		{
			text:     "https://www.google.com",
			expected: "SimpleHTTPRackID",
		},
		/*
			Starting from go 1.17, the FixedRackID function starts to be inlined
			by the compiler, so these tests no longer pass.
			The new full function name is either
			"github.com/reddit/baseplate.go/kafkabp.(*RackIDFunc).UnmarshalText.func1"
			or
			"github.com/reddit/baseplate.go/kafkabp.(*RackIDFunc).UnmarshalText.func2"
			But since inlining is something unpredictable, we no longer test those
			cases.

			{
				text:     "",
				expected: "kafkabp.FixedRackID",
			},
			{
				text:     "aws:foo",
				expected: "kafkabp.FixedRackID",
			},
			{
				text:     "fancy",
				expected: "kafkabp.FixedRackID",
			},
			{
				text:     "http:rack-id",
				expected: "kafkabp.FixedRackID",
			},
		*/
	} {
		t.Run(c.text, func(t *testing.T) {
			var r kafkabp.RackIDFunc
			err := r.UnmarshalText([]byte(c.text))
			if err != nil {
				t.Errorf("Expected UnmarshalText to return nil error, got %v", err)
			}
			name := getActualFuncName(r)
			if !strings.Contains(name, c.expected) {
				t.Errorf("Expected function name to contain %q, got %q", c.expected, name)
			}
		})
	}
}
