package timebp

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
	"time"
)

type jsonTestTypeSecondF struct {
	Timestamp TimestampSecondF `json:"timestamp,omitempty"`
}

type randomSecondFloat float64

func (randomSecondFloat) Generate(r *rand.Rand, _ int) reflect.Value {
	return reflect.ValueOf(randomSecondFloat(
		float64(r.Int63()) / float64(time.Second/secondFRound),
	))
}

func TestTimestampSecondFQuick(t *testing.T) {
	f := func(s randomSecondFloat) bool {
		sec := float64(s)
		timesec := SecondsToTimeF(sec)
		actual := TimeToSecondsF(timesec)
		if !floatsEqual(actual, sec) {
			t.Errorf(
				"For timestamp %v we got %v and %v", sec, timesec, actual,
			)
			return false
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestTimestampSecondFJSONQuick(t *testing.T) {
	f := func(f randomSecondFloat) bool {
		sec := float64(f)

		var v1, v2 jsonTestTypeSecondF

		v1 = jsonTestTypeSecondF{
			Timestamp: TimestampSecondF(SecondsToTimeF(sec)),
		}
		s, err := json.Marshal(v1)
		if err != nil {
			t.Error(err)
			return false
		}
		substr := formatSecondF(sec)
		if !strings.Contains(string(s), substr) {
			t.Errorf("Encoded json expected to contain %q, got %q", substr, s)
		}

		err = json.Unmarshal(s, &v2)
		if err != nil {
			t.Error(err)
			return false
		}
		if !v2.Timestamp.ToTime().Equal(v1.Timestamp.ToTime()) {
			t.Errorf("Expected timestamp %v, got %v", v1.Timestamp, v2.Timestamp)
		}
		return !t.Failed()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestTimestampSecondFJSON(t *testing.T) {
	var v jsonTestTypeSecondF

	t.Run(
		"null",
		func(t *testing.T) {
			s, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			substr := "null"
			if !strings.Contains(string(s), substr) {
				t.Errorf("Encoded json expected to contain %q, got %q", substr, s)
			}
			expectedStr := `{"timestamp":null}`
			if string(s) != expectedStr {
				t.Errorf("Encoded json expected to be %q, got %q", expectedStr, s)
			}

			v.Timestamp = TimestampSecondF(time.Now())
			err = json.Unmarshal(s, &v)
			if err != nil {
				t.Fatal(err)
			}
			if !v.Timestamp.ToTime().IsZero() {
				t.Errorf("Timestamp expected to be zero, got %v", v.Timestamp)
			}
		},
	)

	label := "2019-12-31T23:59:59.123456Z"
	t.Run(
		label,
		func(t *testing.T) {
			const substr = "1577836799.123456"

			ts, err := time.Parse(time.RFC3339Nano, label)
			if err != nil {
				t.Fatal(err)
			}
			v.Timestamp = TimestampSecondF(ts)
			s, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(s), substr) {
				t.Errorf("Encoded json expected to contain %q, got %q", substr, s)
			}
			expectedStr := `{"timestamp":` + substr + `}`
			if string(s) != expectedStr {
				t.Errorf("Encoded json expected to be %q, got %q", expectedStr, s)
			}

			v.Timestamp = TimestampSecondF(time.Now())
			err = json.Unmarshal(s, &v)
			if err != nil {
				t.Fatal(err)
			}
			if !v.Timestamp.ToTime().Equal(ts) {
				t.Errorf("Timestamp expected %v, got %v", ts, v.Timestamp)
			}
		},
	)
}

func TestFormatSecondF(t *testing.T) {
	for _, cc := range []struct {
		sec      float64
		expected string
	}{
		{
			sec:      1577836799,
			expected: "1577836799",
		},
		{
			sec:      1577836799.123,
			expected: "1577836799.123",
		},
		{
			sec:      1577836799.123456,
			expected: "1577836799.123456",
		},
		{
			sec:      1577836700,
			expected: "1577836700",
		},
		{
			sec:      0,
			expected: "0",
		},
		{
			sec:      -1.23456,
			expected: "-1.23456",
		},
	} {
		c := cc
		t.Run(
			c.expected,
			func(t *testing.T) {
				t.Parallel()

				actual := formatSecondF(c.sec)
				if actual != c.expected {
					t.Errorf(
						"formatSecondF(%v) expected %q, got %q",
						c.sec,
						c.expected,
						actual,
					)
				}
			},
		)
	}
}
