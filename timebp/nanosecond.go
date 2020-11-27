package timebp

import (
	"encoding/json"
	"strconv"
	"time"
)

var (
	_ json.Marshaler   = TimestampNanosecond{}
	_ json.Unmarshaler = (*TimestampNanosecond)(nil)
	_ json.Marshaler   = DurationNanosecond(0)
	_ json.Unmarshaler = (*DurationNanosecond)(nil)
)

const nanosecondsPerSecond = int64(time.Second / time.Nanosecond)

// TimestampNanosecond implements json encoding/decoding using nanoseconds
// since EPOCH.
type TimestampNanosecond time.Time

func (ts TimestampNanosecond) String() string {
	return ts.ToTime().String()
}

// ToTime converts TimestampNanosecond back to time.Time.
func (ts TimestampNanosecond) ToTime() time.Time {
	return time.Time(ts)
}

// MarshalJSON implements json.Marshaler interface, using epoch nanoseconds.
func (ts TimestampNanosecond) MarshalJSON() ([]byte, error) {
	t := ts.ToTime()
	if t.IsZero() {
		return []byte("null"), nil
	}

	return []byte(strconv.FormatInt(TimeToNanoseconds(t), 10)), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (ts *TimestampNanosecond) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		*ts = TimestampNanosecond{}
		return nil
	}

	us, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*ts = TimestampNanosecond(NanosecondsToTime(us))
	return nil
}

// NanosecondsToTime converts milliseconds since EPOCH to time.Time.
func NanosecondsToTime(us int64) time.Time {
	if us == 0 {
		return time.Time{}
	}
	// NOTE: A timestamp before the year 1678 or after 2262 would overflow this,
	// but that's OK.
	return time.Unix(
		us/nanosecondsPerSecond,                         // sec
		us%nanosecondsPerSecond*int64(time.Nanosecond), // nanosec
	)
}

// TimeToNanoseconds converts time.Time to nanoseconds since EPOCH.
func TimeToNanoseconds(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	us := t.Unix() * nanosecondsPerSecond
	us += int64(t.Nanosecond()) / int64(time.Nanosecond)
	return us
}

// DurationNanosecond implements json encoding/decoding using nanoseconds
// as int.
type DurationNanosecond time.Duration

func (zd DurationNanosecond) String() string {
	return zd.ToDuration().String()
}

// ToDuration converts DurationNanosecond back to time.Duration.
func (zd DurationNanosecond) ToDuration() time.Duration {
	return time.Duration(zd)
}

// MarshalJSON implements json.Marshaler interface, using nanoseconds.
func (zd DurationNanosecond) MarshalJSON() ([]byte, error) {
	if zd == 0 {
		return []byte("null"), nil
	}
	n := int64(zd.ToDuration() / time.Nanosecond)
	return []byte(strconv.FormatInt(n, 10)), nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (zd *DurationNanosecond) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		*zd = 0
		return nil
	}

	d, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*zd = DurationNanosecond(time.Duration(d) * time.Nanosecond)
	return nil
}
