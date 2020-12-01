package timebp

import (
	"encoding/json"
	"strconv"
	"time"
)

var (
	_ json.Unmarshaler = (*TimestampMillisecond)(nil)
	_ json.Marshaler   = TimestampMillisecond{}
)

const millisecondsPerSecond = int64(time.Second / time.Millisecond)

// TimestampMillisecond implements json encoding/decoding using milliseconds
// since EPOCH.
type TimestampMillisecond time.Time

func (ts TimestampMillisecond) String() string {
	return ts.ToTime().String()
}

// ToTime converts TimestampMillisecond back to time.Time.
func (ts TimestampMillisecond) ToTime() time.Time {
	return time.Time(ts)
}

// UnmarshalJSON implements json.Unmarshaler.
func (ts *TimestampMillisecond) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		*ts = TimestampMillisecond{}
		return nil
	}

	ms, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*ts = TimestampMillisecond(MillisecondsToTime(ms))
	return nil
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

// MarshalJSON implements json.Marshaler.
func (ts TimestampMillisecond) MarshalJSON() ([]byte, error) {
	t := ts.ToTime()
	if t.IsZero() {
		return []byte("null"), nil
	}

	return []byte(strconv.FormatInt(TimeToMilliseconds(t), 10)), nil
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
