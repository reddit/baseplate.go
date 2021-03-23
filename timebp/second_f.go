package timebp

import (
	"encoding"
	"encoding/json"
	"math"
	"strconv"
	"time"
)

var (
	_ json.Unmarshaler         = (*TimestampSecondF)(nil)
	_ json.Marshaler           = TimestampSecondF{}
	_ encoding.TextUnmarshaler = (*TimestampSecondF)(nil)
	_ encoding.TextMarshaler   = TimestampSecondF{}
)

// float64 does not really have the nanosecond precision for a timestamp around
// year 2020, so we only keep the precision up to microseconds.
const secondFRound = time.Microsecond

// 1 microsecond is 1e-6 seconds.
const epsilon = 1e-6

func floatsEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

// TimestampSecondF implements json/text encoding/decoding using float number
// seconds since EPOCH, with precision up to microseconds.
type TimestampSecondF time.Time

func (ts TimestampSecondF) String() string {
	return ts.ToTime().String()
}

// ToTime converts TimestampSecondF back to time.Time.
func (ts TimestampSecondF) ToTime() time.Time {
	return time.Time(ts).Round(secondFRound)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (ts *TimestampSecondF) UnmarshalText(data []byte) error {
	// Empty/default
	if len(data) == 0 {
		*ts = TimestampSecondF{}
		return nil
	}

	sec, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return err
	}
	*ts = TimestampSecondF(SecondsToTimeF(sec))
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (ts *TimestampSecondF) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		*ts = TimestampSecondF{}
		return nil
	}

	return ts.UnmarshalText(data)
}

// SecondsToTimeF converts float seconds since EPOCH to time.Time.
func SecondsToTimeF(s float64) time.Time {
	if floatsEqual(s, 0) {
		return time.Time{}
	}
	sec := int64(s)
	nanosec := (s - float64(sec)) * float64(time.Second)
	return time.Unix(sec, int64(nanosec))
}

// MarshalText implements encoding.TextMarshaler.
func (ts TimestampSecondF) MarshalText() ([]byte, error) {
	t := ts.ToTime()
	if t.IsZero() {
		return nil, nil
	}

	return []byte(formatSecondF(TimeToSecondsF(t))), nil
}

// MarshalJSON implements json.Marshaler.
func (ts TimestampSecondF) MarshalJSON() ([]byte, error) {
	t := ts.ToTime()
	if t.IsZero() {
		return []byte("null"), nil
	}

	return ts.MarshalText()
}

// TimeToSecondsF converts time.Time to float seconds since EPOCH.
func TimeToSecondsF(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}
	t = t.Round(secondFRound)
	sec := float64(t.Unix())
	sec += float64(t.Nanosecond()) / float64(time.Second)
	return sec
}

func formatSecondF(sec float64) string {
	return strconv.FormatFloat(sec, 'f', -1, 64)
}
