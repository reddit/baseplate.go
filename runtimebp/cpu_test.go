package runtimebp

import (
	"io"
	"math"
	"os"
	"strings"
	"testing"
)

func TestReadNumbersFromFile(t *testing.T) {
	buf := make([]byte, 1024)

	compareFloat64Slices := func(t *testing.T, want, got []float64) {
		t.Helper()
		t.Logf("compareFloat64Slices: want %v got %v", want, got)
		if len(want) != len(got) {
			t.Errorf(
				"Slice length mismatch: want %d got %d",
				len(want),
				len(got),
			)
			return
		}
		for i, fWant := range want {
			fGot := got[i]
			if math.Abs(fWant-fGot) > 1e-5 {
				t.Errorf("#%d want %f, got %f", i, fWant, fGot)
			}
		}
	}

	writeFile := func(t *testing.T, content string) string {
		t.Helper()

		dir := t.TempDir()
		file, err := os.CreateTemp(dir, "")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}()
		if _, err := io.Copy(file, strings.NewReader(content)); err != nil {
			t.Fatal(err)
		}
		return file.Name()
	}

	cases := map[string]struct {
		Content  string
		Numbers  int
		Error    bool
		Expected []float64
	}{
		"normal-v1": {
			Content:  "15000",
			Numbers:  1,
			Expected: []float64{15000},
		},
		"normal-v2": {
			Content:  "15000 1000",
			Numbers:  2,
			Expected: []float64{15000, 1000},
		},
		"v2-line-break": {
			Content:  "15000\n1000",
			Numbers:  2,
			Expected: []float64{15000, 1000},
		},
		"v2-max": {
			Content: "max 1000",
			Numbers: 2,
			Error:   true,
		},
		"with-line-break": {
			Content:  "15000\n",
			Numbers:  1,
			Expected: []float64{15000},
		},
		"with-extra-data": {
			Content: "15000foobar",
			Numbers: 1,
			Error:   true,
		},
		"total-garbage": {
			Content: "Hello, world!",
			Numbers: 1,
			Error:   true,
		},
		"not-int": {
			Content: "123.456",
			Numbers: 1,
			Error:   true,
		},
		"negative": {
			Content:  "-1",
			Numbers:  1,
			Expected: []float64{-1},
		},
		"mismatch-numbers-1": {
			Content: "15000",
			Numbers: 2,
			Error:   true,
		},
		"mismatch-numbers-2": {
			Content: "15000 1000",
			Numbers: 1,
			Error:   true,
		},
	}

	for label, data := range cases {
		t.Run(label, func(t *testing.T) {
			path := writeFile(t, data.Content)
			t.Cleanup(func() {
				os.Remove(path)
			})

			values, err := readNumbersFromFile(path, buf, data.Numbers)
			if data.Error {
				if err == nil {
					t.Errorf("Expected an error for %+v, got nil", data)
				}
			} else {
				if err != nil {
					t.Fatalf("Got error for %+v: %v", data, err)
				}
				compareFloat64Slices(t, data.Expected, values)
			}
		})
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
		t.Run(label, func(t *testing.T) {
			actual := boundNtoMinMax(data.N, data.Min, data.Max)
			if actual != data.Expected {
				t.Errorf("Got %d for data %+v", actual, data)
			}
		})
	}
}

func TestFetchCPURequest(t *testing.T) {
	cases := map[string]struct {
		N        string
		Expected int
		Ok       bool
	}{
		"plainInt": {
			N:        "3",
			Expected: 6,
			Ok:       true,
		},
		"goodMilliLessThanOne": {
			N:        "500m",
			Expected: 2,
			Ok:       true,
		},
		"goodMilliGreaterThanOne": {
			N:        "1400m",
			Expected: 3,
			Ok:       true,
		},
		"badMilli": {
			N:        "0m",
			Expected: 2,
			Ok:       true,
		},
		"noMilli": {
			N:        "",
			Expected: 0,
			Ok:       false,
		},
	}

	for label, data := range cases {
		t.Run(label, func(t *testing.T) {
			if data.N == "" {
				os.Unsetenv("BASEPLATE_CPU_REQUEST")
			} else {
				os.Setenv("BASEPLATE_CPU_REQUEST", data.N)
				defer os.Unsetenv("BASEPLATE_CPU_REQUEST")
			}
			actual, ok := fetchCPURequest()
			if actual != data.Expected {
				t.Errorf("Got %d for data %+v", actual, data)
			}
			if data.Ok != ok {
				t.Errorf("Got %v for ok check on %+v", ok, data)
			}
		})
	}
}
