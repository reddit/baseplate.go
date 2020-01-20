package timebp

import (
	"encoding/json"
	"strconv"
	"time"
)

var (
	_ json.Marshaler   = TimestampMicrosecond{}
	_ json.Unmarshaler = (*TimestampMicrosecond)(nil)
	_ json.Marshaler   = DurationMicrosecond(0)
	_ json.Unmarshaler = (*DurationMicrosecond)(nil)
)

const microsecondsPerSecond = int64(time.Second / time.Microsecond)

// TimestampMicrosecond implements json encoding/decoding using microseconds
// since EPOCH.
type TimestampMicrosecond time.Time

func (ts TimestampMicrosecond) String() string {
	return ts.ToTime().String()
}

// ToTime converts TimestampMicrosecond back to time.Time.
func (ts TimestampMicrosecond) ToTime() time.Time {
	return time.Time(ts)
}

// MarshalJSON implements json.Marshaler interface, using epoch microseconds.
func (ts TimestampMicrosecond) MarshalJSON() ([]byte, error) {
	t := ts.ToTime()
	if t.IsZero() {
		return []byte("null"), nil
	}

	return []byte(strconv.FormatInt(TimeToMicroseconds(t), 10)), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (ts *TimestampMicrosecond) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		*ts = TimestampMicrosecond{}
		return nil
	}

	us, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*ts = TimestampMicrosecond(MicrosecondsToTime(us))
	return nil
}

// MicrosecondsToTime converts milliseconds since EPOCH to time.Time.
func MicrosecondsToTime(us int64) time.Time {
	if us == 0 {
		return time.Time{}
	}
	// NOTE: A timestamp before the year 1678 or after 2262 would overflow this,
	// but that's OK.
	return time.Unix(
		us/microsecondsPerSecond,                         // sec
		us%microsecondsPerSecond*int64(time.Microsecond), // nanosec
	)
}

// TimeToMicroseconds converts time.Time to microseconds since EPOCH.
func TimeToMicroseconds(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	us := t.Unix() * microsecondsPerSecond
	us += int64(t.Nanosecond()) / int64(time.Microsecond)
	return us
}

// DurationMicrosecond implements json encoding/decoding using microseconds
// as int.
type DurationMicrosecond time.Duration

func (zd DurationMicrosecond) String() string {
	return zd.ToDuration().String()
}

// ToDuration converts DurationMicrosecond back to time.Duration.
func (zd DurationMicrosecond) ToDuration() time.Duration {
	return time.Duration(zd)
}

// MarshalJSON implements json.Marshaler interface, using microseconds.
func (zd DurationMicrosecond) MarshalJSON() ([]byte, error) {
	if zd == 0 {
		return []byte("null"), nil
	}
	n := int64(zd.ToDuration() / time.Microsecond)
	return []byte(strconv.FormatInt(n, 10)), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (zd *DurationMicrosecond) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		*zd = 0
		return nil
	}

	d, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*zd = DurationMicrosecond(time.Duration(d) * time.Microsecond)
	return nil
}
