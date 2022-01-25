package log_test

import (
	"bytes"
	"context"
	"encoding"
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
)

// ExtendedLogWrapper extends log.Wrapper to support metricsbp.LogWrapper.
type ExtendedLogWrapper struct {
	log.Wrapper
}

var st = metricsbp.NewStatsd(
	context.Background(),
	metricsbp.Config{
		// This is to make sure that when we read the metricsbp statsd buffer out
		// manually later in the example they are not already sent to a blackhole
		// sink.
		// This is NOT needed in production code as in production code the sink
		// shall be configured as the actual statsd collector and not read out
		// manually like in this example.
		BufferInMemoryForTesting: true,
	},
)

// UnmarshalText implements encoding.TextUnmarshaler.
//
// In addition to the implementations log.Wrapper.UnmarshalText supports, it
// added supports for:
//
// - "counter:metrics:logger": metricsbp.LogWrapper, with "metrics" being the
// name of the counter metrics and "logger" being the underlying logger.
func (e *ExtendedLogWrapper) UnmarshalText(text []byte) error {
	const counterPrefix = "counter:"
	if s := string(text); strings.HasPrefix(s, counterPrefix) {
		parts := strings.SplitN(s, ":", 3)
		if len(parts) != 3 {
			return fmt.Errorf("unsupported log.Wrapper config: %q", text)
		}
		var w log.Wrapper
		if err := w.UnmarshalText([]byte(parts[2])); err != nil {
			return err
		}
		e.Wrapper = metricsbp.LogWrapper(metricsbp.LogWrapperArgs{
			Counter: parts[1],
			Wrapper: w,
			Statsd:  st,
		})
		return nil
	}
	return e.Wrapper.UnmarshalText(text)
}

func (e ExtendedLogWrapper) ToLogWrapper() log.Wrapper {
	return e.Wrapper
}

var _ encoding.TextUnmarshaler = (*ExtendedLogWrapper)(nil)

// This example demonstrates how to write your own type to "extend"
// log.Wrapper.UnmarshalText to add other implementations.
func ExampleWrapper_UnmarshalText() {
	const (
		invalid     = `logger: fancy`
		counterOnly = `logger: "counter:foo.bar.count:nop"`
	)
	var data struct {
		Logger ExtendedLogWrapper `yaml:"logger"`
	}

	fmt.Printf(
		"This is an invalid config: %s, err: %v\n",
		invalid,
		yaml.Unmarshal([]byte(invalid), &data),
	)

	fmt.Printf(
		"This is an counter-only config: %s, err: %v\n",
		counterOnly,
		yaml.Unmarshal([]byte(counterOnly), &data),
	)
	data.Logger.Log(context.Background(), "Hello, world!")
	var buf bytes.Buffer
	st.WriteTo(&buf)
	fmt.Printf("Counter: %s", buf.String())

	// Output:
	// This is an invalid config: logger: fancy, err: unsupported log.Wrapper config: "fancy"
	// This is an counter-only config: logger: "counter:foo.bar.count:nop", err: <nil>
	// Counter: foo.bar.count:1.000000|c
}
