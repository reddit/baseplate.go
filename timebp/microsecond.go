package timebp

import (
	"encoding"
	"encoding/json"
	"strconv"
	"time"
)

var (
	_ json.Marshaler           = TimestampMicrosecond{}
	_ json.Unmarshaler         = (*TimestampMicrosecond)(nil)
	_ encoding.TextMarshaler   = TimestampMicrosecond{}
	_ encoding.TextUnmarshaler = (*TimestampMicrosecond)(nil)

	_ json.Marshaler           = DurationMicrosecond(0)
	_ json.Unmarshaler         = (*DurationMicrosecond)(nil)
	_ encoding.TextMarshaler   = DurationMicrosecond(0)
	_ encoding.TextUnmarshaler = (*DurationMicrosecond)(nil)
)

// TimestampMicrosecond implements json/text encoding/decoding using
// microseconds since EPOCH.
type TimestampMicrosecond time.Time

func (ts TimestampMicrosecond) String() string {
	return ts.ToTime().String()
}

// ToTime converts TimestampMicrosecond back to time.Time.
func (ts TimestampMicrosecond) ToTime() time.Time {
	return time.Time(ts)
}

// MarshalText implemnts encoding.TextMarshaler.
func (ts TimestampMicrosecond) MarshalText() ([]byte, error) {
	t := ts.ToTime()
	if t.IsZero() {
		return nil, nil
	}

	return []byte(strconv.FormatInt(TimeToMicroseconds(t), 10)), nil
}

// MarshalJSON implements json.Marshaler interface, using epoch microseconds.
func (ts TimestampMicrosecond) MarshalJSON() ([]byte, error) {
	t := ts.ToTime()
	if t.IsZero() {
		return []byte("null"), nil
	}

	return ts.MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (ts *TimestampMicrosecond) UnmarshalText(data []byte) error {
	// Empty/default
	if len(data) == 0 {
		*ts = TimestampMicrosecond{}
		return nil
	}

	us, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*ts = TimestampMicrosecond(MicrosecondsToTime(us))
	return nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (ts *TimestampMicrosecond) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*ts = TimestampMicrosecond{}
		return nil
	}

	return ts.UnmarshalText(data)
}

// MicrosecondsToTime converts milliseconds since EPOCH to time.Time.
func MicrosecondsToTime(us int64) time.Time {
	if us == 0 {
		return time.Time{}
	}
	return time.UnixMicro(us)
}

// TimeToMicroseconds converts time.Time to microseconds since EPOCH.
func TimeToMicroseconds(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMicro()
}

// DurationMicrosecond implements json/text encoding/decoding using microseconds
// as int.
type DurationMicrosecond time.Duration

func (zd DurationMicrosecond) String() string {
	return zd.ToDuration().String()
}

// ToDuration converts DurationMicrosecond back to time.Duration.
func (zd DurationMicrosecond) ToDuration() time.Duration {
	return time.Duration(zd)
}

// MarshalText implements encoding.TextMarshaler.
func (zd DurationMicrosecond) MarshalText() ([]byte, error) {
	n := int64(zd.ToDuration() / time.Microsecond)
	return []byte(strconv.FormatInt(n, 10)), nil
}

// MarshalJSON implements json.Marshaler interface, using microseconds.
func (zd DurationMicrosecond) MarshalJSON() ([]byte, error) {
	if zd == 0 {
		return []byte("null"), nil
	}
	return zd.MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (zd *DurationMicrosecond) UnmarshalText(data []byte) error {
	// Empty/default
	if len(data) == 0 {
		*zd = 0
		return nil
	}

	d, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*zd = DurationMicrosecond(time.Duration(d) * time.Microsecond)
	return nil
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (zd *DurationMicrosecond) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		*zd = 0
		return nil
	}

	return zd.UnmarshalText(data)
}
