package metricsbp_test

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/metricsbp"
)

type inner struct {
	A io.Reader
	B io.Reader
}

type test struct {
	A         io.Reader
	b         io.Reader
	C         io.Reader
	Inner     inner
	Anonymous *struct {
		A io.Reader
		B io.Reader
	}
}

func TestCheckNilFields(t *testing.T) {
	reader := strings.NewReader("")
	v := &test{
		A: reader,
		b: nil, // should be reported
		C: nil, // should be reported
		Inner: inner{
			A: nil, // should be reported
			B: reader,
		},
		Anonymous: nil, // should be reported
	}
	expected := []string{
		"*test.b",
		"*test.C",
		"*test.Inner.A",
		"*test.Anonymous",
	}
	actual := metricsbp.CheckNilFields(v)
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf(
			"For metricsbp.CheckNilFields(%+v), expected result %+v, got %+v",
			v,
			expected,
			actual,
		)
	}
}

func TestCheckNilFieldsNil(t *testing.T) {
	var v *test
	expected := []string{""}
	actual := metricsbp.CheckNilFields(v)
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf(
			"For metricsbp.CheckNilFields(%+v), expected result %+v, got %+v",
			v,
			expected,
			actual,
		)
	}
}
