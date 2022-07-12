package configbp_test

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"testing/quick"

	"gopkg.in/yaml.v2"

	"github.com/reddit/baseplate.go/configbp"
)

type config struct {
	I64 configbp.Int64String `yaml:"i64"`
}

func checkValid(tb testing.TB, i int64) {
	s := fmt.Sprintf(`i64: "%d"`, i)
	var cfg config
	decoder := yaml.NewDecoder(strings.NewReader(s))
	decoder.SetStrict(true)
	if err := decoder.Decode(&cfg); err != nil {
		tb.Errorf("Failed to unmarshal yaml: %v", err)
	}
	if int64(cfg.I64) != i {
		tb.Errorf("got %d, want %d", cfg.I64, i)
	}

	s = fmt.Sprintf(`i64: %d`, i)
	cfg = config{}
	decoder = yaml.NewDecoder(strings.NewReader(s))
	decoder.SetStrict(true)
	if err := decoder.Decode(&cfg); err != nil {
		tb.Errorf("Failed to unmarshal yaml: %v", err)
	}
	if int64(cfg.I64) != i {
		tb.Errorf("got %d, want %d", cfg.I64, i)
	}
}

func checkLikelyInvalid(tb testing.TB, str string) {
	_, wantErr := strconv.ParseInt(str, 10, 64)
	s := fmt.Sprintf(`i64: %q`, str)
	var cfg config
	decoder := yaml.NewDecoder(strings.NewReader(s))
	decoder.SetStrict(true)
	err := decoder.Decode(&cfg)
	if wantErr == nil && err != nil {
		tb.Errorf("got %v, want nil", err)
	}
	if wantErr != nil && err == nil {
		tb.Errorf("got nil, want %v", wantErr)
	}
}

func TestInt64StringValid(t *testing.T) {
	f := func(i int64) bool {
		checkValid(t, i)
		return !t.Failed()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestInt64StringInvalid(t *testing.T) {
	f := func(s string) bool {
		checkLikelyInvalid(t, s)
		return !t.Failed()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func FuzzInt64StringValid(f *testing.F) {
	for _, input := range []int64{
		0,
		1,
		-1,
		math.MaxInt64,
		math.MinInt64,
	} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, i int64) {
		checkValid(t, i)
	})
}

func FuzzInt64StringInvalid(f *testing.F) {
	for _, input := range []string{
		"",
		"not int64",
		"foo_bar",
		"1",
		"-1",
		strconv.FormatInt(math.MaxInt64, 10),
	} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, s string) {
		checkLikelyInvalid(t, s)
	})
}
