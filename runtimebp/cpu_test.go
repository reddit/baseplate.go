package runtimebp

import (
	"math"
	"os"
	"testing"
)

func TestReadNumberFromFile(t *testing.T) {
	buf := make([]byte, 1024)

	compareFloat64 := func(t *testing.T, expected, actual float64) {
		t.Helper()
		if math.Abs(expected-actual) > 1e-5 {
			t.Errorf("Expected %f, got %f", expected, actual)
		}
	}

	writeFile := func(t *testing.T, content string) string {
		t.Helper()

		file, err := os.CreateTemp("", "")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}()
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		return file.Name()
	}

	cases := map[string]struct {
		Content  string
		Error    bool
		Expected float64
	}{
		"normal": {
			Content:  "15000",
			Expected: 15000,
		},
		"with-line-break": {
			Content:  "15000\n",
			Expected: 15000,
		},
		"with-extra-data": {
			Content: "15000foobar",
			Error:   true,
		},
		"total-garbage": {
			Content: "Hello, world!",
			Error:   true,
		},
		"not-int": {
			Content: "123.456",
			Error:   true,
		},
		"negative": {
			Content:  "-1",
			Expected: -1,
		},
	}

	for label, data := range cases {
		t.Run(
			label,
			func(t *testing.T) {
				path := writeFile(t, data.Content)
				defer os.Remove(path)
				f, err := readNumberFromFile(path, buf)
				if data.Error {
					if err == nil {
						t.Errorf("Expected an error for %+v, got nil", data)
					}
				} else {
					if err != nil {
						t.Fatalf("Got error for %+v: %v", data, err)
					}
					compareFloat64(t, data.Expected, f)
				}
			},
		)
	}
}

func TestBoundNtoMinMax(t *testing.T) {
	cases := map[string]struct {
		N, Min, Max, Expected int
	}{
		"normal": {
			N:        3,
			Min:      1,
			Max:      5,
			Expected: 3,
		},
		"min": {
			N:        1,
			Min:      1,
			Max:      5,
			Expected: 1,
		},
		"max": {
			N:        5,
			Min:      1,
			Max:      5,
			Expected: 5,
		},
		"less-than-min": {
			N:        0,
			Min:      1,
			Max:      5,
			Expected: 1,
		},
		"more-than-max": {
			N:        10,
			Min:      1,
			Max:      5,
			Expected: 5,
		},
		"NaN": {
			N:        int(math.NaN()),
			Min:      1,
			Max:      5,
			Expected: 1,
		},
	}

	for label, data := range cases {
		t.Run(
			label,
			func(t *testing.T) {
				actual := boundNtoMinMax(data.N, data.Min, data.Max)
				if actual != data.Expected {
					t.Errorf("Got %d for data %+v", actual, data)
				}
			},
		)
	}
}
