package timebp

import (
	"encoding"
	"encoding/json"
	"strconv"
	"time"
)

var (
	_ json.Unmarshaler         = (*TimestampMillisecond)(nil)
	_ json.Marshaler           = TimestampMillisecond{}
	_ encoding.TextUnmarshaler = (*TimestampMillisecond)(nil)
	_ encoding.TextMarshaler   = TimestampMillisecond{}
)

const millisecondsPerSecond = int64(time.Second / time.Millisecond)

// TimestampMillisecond implements json/text encoding/decoding using
// milliseconds since EPOCH.
type TimestampMillisecond time.Time

func (ts TimestampMillisecond) String() string {
	return ts.ToTime().String()
}

// ToTime converts TimestampMillisecond back to time.Time.
func (ts TimestampMillisecond) ToTime() time.Time {
	return time.Time(ts)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (ts *TimestampMillisecond) UnmarshalText(data []byte) error {
	// Empty/default
	if len(data) == 0 {
		*ts = TimestampMillisecond{}
		return nil
	}

	ms, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*ts = TimestampMillisecond(MillisecondsToTime(ms))
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (ts *TimestampMillisecond) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*ts = TimestampMillisecond{}
		return nil
	}
	return ts.UnmarshalText(data)
}

// MillisecondsToTime converts milliseconds since EPOCH to time.Time.
func MillisecondsToTime(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.Unix(
		ms/millisecondsPerSecond,                         // sec
		ms%millisecondsPerSecond*int64(time.Millisecond), // nanosec
	)
}

// MarshalText implements encoding.TextMarshaler.
func (ts TimestampMillisecond) MarshalText() ([]byte, error) {
	t := ts.ToTime()
	if t.IsZero() {
		return nil, nil
	}

	return []byte(strconv.FormatInt(TimeToMilliseconds(t), 10)), nil
}

// MarshalJSON implements json.Marshaler.
func (ts TimestampMillisecond) MarshalJSON() ([]byte, error) {
	t := ts.ToTime()
	if t.IsZero() {
		return []byte("null"), nil
	}

	return ts.MarshalText()
}

// TimeToMilliseconds converts time.Time to milliseconds since EPOCH.
func TimeToMilliseconds(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	ms := t.Unix() * millisecondsPerSecond
	ms += int64(t.Nanosecond()) / int64(time.Millisecond)
	return ms
}
