package timebp_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/reddit/baseplate.go/timebp"
)

type jsonTestTypeMicrosecond struct {
	Timestamp timebp.TimestampMicrosecond `json:"timestamp,omitempty"`
	Duration  timebp.DurationMicrosecond  `json:"duration,omitempty"`
}

func TestTimestampMicrosecondQuick(t *testing.T) {
	f := func(us int64) bool {
		time := timebp.MicrosecondsToTime(us)
		actual := timebp.TimeToMicroseconds(time)
		if actual != us {
			t.Errorf(
				"For timestamp %d we got %v and %d", us, time, actual,
			)
			return false
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestTimestampJSONQuick(t *testing.T) {
	f := func(us int64, duration time.Duration) bool {
		var v1, v2 jsonTestTypeMicrosecond

		v1 = jsonTestTypeMicrosecond{
			Timestamp: timebp.TimestampMicrosecond(timebp.MicrosecondsToTime(us)),
			Duration:  timebp.DurationMicrosecond(duration.Round(time.Microsecond)),
		}
		s, err := json.Marshal(v1)
		if err != nil {
			t.Error(err)
			return false
		}
		substr := strconv.FormatInt(us, 10)
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
		if v2.Duration != v1.Duration {
			t.Errorf("Expected duration %v, got %v", v1.Duration, v2.Duration)
		}
		return !t.Failed()
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

func TestTimestampJSON(t *testing.T) {
	var v jsonTestTypeMicrosecond

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

			v.Timestamp = timebp.TimestampMicrosecond(time.Now())
			err = json.Unmarshal(s, &v)
			if err != nil {
				t.Fatal(err)
			}
			if !v.Timestamp.ToTime().IsZero() {
				t.Errorf("Timestamp expected to be zero, got %v", v.Timestamp)
			}
			if v.Duration != 0 {
				t.Errorf("Duration expected to be zero, got %v", v.Duration)
			}
		},
	)

	label := "2019-12-31T23:59:59.123456Z"
	t.Run(
		label,
		func(t *testing.T) {
			const substr = "1577836799123456"

			ts, err := time.Parse(time.RFC3339Nano, label)
			if err != nil {
				t.Fatal(err)
			}
			v.Timestamp = timebp.TimestampMicrosecond(ts)
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

			v.Timestamp = timebp.TimestampMicrosecond(time.Now())
			err = json.Unmarshal(s, &v)
			if err != nil {
				t.Fatal(err)
			}
			if !v.Timestamp.ToTime().Equal(ts) {
				t.Errorf("Timestamp expected %v, got %v", ts, v.Timestamp)
			}
		},
	)

	label = "10s"
	t.Run(
		label,
		func(t *testing.T) {
			const substr = "10000000"
			const expectedStr = `{"timestamp":1577836799123456,"duration":10000000}`

			duration, err := time.ParseDuration(label)
			if err != nil {
				t.Fatal(err)
			}
			v.Duration = timebp.DurationMicrosecond(duration)
			s, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(s), substr) {
				t.Errorf("Encoded json expected to contain %q, got %q", substr, s)
			}
			if string(s) != expectedStr {
				t.Errorf("Encoded json expected to be %q, got %q", expectedStr, s)
			}

			v.Duration = 0
			err = json.Unmarshal(s, &v)
			if err != nil {
				t.Fatal(err)
			}
			if v.Duration.ToDuration() != duration {
				t.Errorf("Duration expected %v, got %v", duration, v.Duration)
			}
		},
	)
}
