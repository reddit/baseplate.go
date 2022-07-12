package kafkabp

import (
	"context"
	"encoding"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/reddit/baseplate.go/log"
)

// RackIDFunc defines a function to provide the kafka rack id to use.
//
// Rack id is not considered a crucial part of kakfa client configuration,
// so this function doesn't have the ability to return any errors.
// If an error occurred while retrieving the rack id,
// the implementation should return empty string and handle the error by itself
// (logging, panic, etc.).
//
// See the following URL for more info regarding kafka's rack awareness feature:
// https://cwiki.apache.org/confluence/display/KAFKA/KIP-36+Rack+aware+replica+assignment
type RackIDFunc func() string

// UnmarshalText implements encoding.TextUnmarshaler.
//
// It makes RackIDFunc possible to be used directly in yaml and other config
// files.
//
// Please note that this currently only support limited implementations:
//
// - empty: Empty rack id (same as "fixed:"). Please note that this might be
// changed to "aws" in the future.
//
// - "fixed:id": FixedRackID with given id. A special case of "fixed:" means no
// rack id.
//
// - "aws": AWSAvailabilityZoneRackID.
//
// - "http://url" or "https://url": SimpleHTTPRackID with
// log.DefaultWrapper and prometheus counter of
// kafkabp_http_rack_id_failure_total, default timeout & limit, and given URL.
//
// - anything else: FixedRackID with the given value. For example "foobar" is
// the same as "fixed:foobar".
func (r *RackIDFunc) UnmarshalText(text []byte) error {
	s := string(text)

	// Simple cases
	switch s {
	case "":
		*r = FixedRackID("")
		return nil
	case "aws":
		*r = AWSAvailabilityZoneRackID
		return nil
	}

	// http cases
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		*r = SimpleHTTPRackID(SimpleHTTPRackIDConfig{
			URL: s,
			Logger: log.CounterWrapper(
				nil, // delegate, let it fallback to DefaultWrapper
				httpRackFailure,
			),
		})
		return nil
	}

	// The rest are all FixedRackID cases, remove "fixed:" prefix if it's there.
	*r = FixedRackID(strings.TrimPrefix(s, "fixed:"))
	return nil
}

var _ encoding.TextUnmarshaler = (*RackIDFunc)(nil)

// FixedRackID is a RackIDFunc implementation that returns a fixed rack id.
func FixedRackID(id string) RackIDFunc {
	return func() string {
		return id
	}
}

// Default values for SimpleHTTPRackIDConfig.
const (
	SimpleHTTPRackIDDefaultLimit   = 1024
	SimpleHTTPRackIDDefaultTimeout = time.Second
)

// SimpleHTTPRackIDConfig defines the config to be used in SimpleHTTPRackID.
type SimpleHTTPRackIDConfig struct {
	// URL to fetch rack id from. Required.
	URL string

	// Limit of how many bytes to read from the response.
	//
	// Optional, default to SimpleHTTPRackIDDefaultLimit.
	Limit int64

	// HTTP client timeout.
	//
	// Optional, default to SimpleHTTPRackIDDefaultTimeout.
	Timeout time.Duration

	// Logger to be used on http errors. Optional.
	Logger log.Wrapper
}

// SimpleHTTPRackID is a RackIDFunc implementation that gets the rack id from an
// HTTP URL.
//
// It's "simple" as in it always treat the HTTP response as plain text (so it
// shouldn't be used for JSON endpoints), read up to Limit bytes, and trim
// leading and trailing spaces before returning. If an HTTP error occurred it
// will be logged using Logger passed in.
func SimpleHTTPRackID(cfg SimpleHTTPRackIDConfig) RackIDFunc {
	if cfg.Limit <= 0 {
		cfg.Limit = SimpleHTTPRackIDDefaultLimit
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = SimpleHTTPRackIDDefaultTimeout
	}

	return func() string {
		client := http.Client{
			Timeout: cfg.Timeout,
		}
		resp, err := client.Get(cfg.URL)
		if err != nil {
			cfg.Logger.Log(context.Background(), fmt.Sprintf(
				"Failed to get rack id from %s: %v",
				cfg.URL,
				err,
			))
			return ""
		}

		defer func() {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()
		content, err := io.ReadAll(io.LimitReader(resp.Body, cfg.Limit))
		if err != nil {
			cfg.Logger.Log(context.Background(), fmt.Sprintf(
				"Failed to read rack id response from %s: %v",
				cfg.URL,
				err,
			))
			return ""
		}
		if resp.StatusCode >= 400 {
			cfg.Logger.Log(context.Background(), fmt.Sprintf(
				"Rack id URL %s returned status code %d: %s",
				cfg.URL,
				resp.StatusCode,
				content,
			))
			return ""
		}
		return strings.TrimSpace(string(content))
	}
}

// Global cache for AWSAvailabilityZoneRackID.
var (
	awsCachedRackID string
	awsRackIDOnce   sync.Once
)

// References:
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-categories.html
const awsAZurl = "http://169.254.169.254/latest/meta-data/placement/availability-zone"

// AWSAvailabilityZoneRackID is a RackIDFunc implementation that returns AWS
// availability zone as the rack id.
//
// It also caches the result globally, so if you have more than one
// AWSAvailabilityZoneRackID in your process only the first one actually makes
// the HTTP request, for example:
//
//    consumer1 := kafkabp.NewConsumer(kafkabp.ConsumerConfig{
//        RackID: kafkabp.AWSAvailabilityZoneRackID,
//        Topic:  "topic1",
//        // other configs
//    })
//    consumer2 := kafkabp.NewConsumer(kafkabp.ConsumerConfig{
//        RackID: kafkabp.AWSAvailabilityZoneRackID,
//        Topic:  "topic2",
//        // other configs
//    })
//
// It uses SimpleHTTPRackIDConfig underneath with log.DefaultWrapper with a
// prometheus counter of kafkabp_aws_rack_id_failure_total and default
// Limit & Timeout.
func AWSAvailabilityZoneRackID() string {
	awsRackIDOnce.Do(func() {
		awsCachedRackID = SimpleHTTPRackID(SimpleHTTPRackIDConfig{
			URL: awsAZurl,
			Logger: log.CounterWrapper(
				nil, // delegate, let it fallback to DefaultWrapper
				awsRackFailure,
			),
		})()
	})
	return awsCachedRackID
}
