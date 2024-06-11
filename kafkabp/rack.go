package kafkabp

import (
	"context"
	"encoding"
	"fmt"
	"io"
	"log/slog"
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
	SimpleHTTPRackIDDefaultTimeout = 1 * time.Second
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
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, cfg.URL, http.MethodGet, http.NoBody)
		if err != nil {
			cfg.Logger.Log(ctx, fmt.Sprintf(
				"Failed to get rack id from %s: %v",
				cfg.URL,
				err,
			))
			return ""
		}

		content, err := doHTTP(req, cfg.Limit)
		if err != nil {
			cfg.Logger.Log(context.Background(), fmt.Sprintf(
				"Failed to get rack id from %s: %v",
				cfg.URL,
				err,
			))
			return ""
		}
		return content
	}
}

var client http.Client

// doHTTP executes http request, reads the body up to the limit given, and
// return the body read as string with whitespace trimmed.
func doHTTP(r *http.Request, readLimit int64) (string, error) {
	resp, err := client.Do(r)
	if err != nil {
		return "", fmt.Errorf("kafkabp.doHTTP: request failed: %w", err)
	}

	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	content, err := io.ReadAll(io.LimitReader(resp.Body, readLimit))
	if err != nil {
		return "", fmt.Errorf("kafkabp.doHTTP: failed to read response body: %w", err)
	}

	body := strings.TrimSpace(string(content))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("kafkabp.doHTTP: got http response with code %d and body %q", resp.StatusCode, body)
	}

	return body, nil
}

var awsRackID = sync.OnceValues(func() (string, error) {
	const (
		// References:
		// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html
		// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-categories.html
		tokenURL = "http://169.254.169.254/latest/api/token"
		azURL    = "http://169.254.169.254/latest/meta-data/placement/availability-zone"

		timeout   = time.Second
		readLimit = 1024
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	token, err := func(ctx context.Context) (string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, tokenURL, http.NoBody)
		if err != nil {
			return "", fmt.Errorf("kafkabp.awsRackID: failed to create request from url %q: %w", tokenURL, err)
		}
		req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "21600")

		token, err := doHTTP(req, readLimit)
		if err != nil {
			return "", fmt.Errorf("kafkabp.awsRackID: failed to get AWS IMDS v2 token from url %q: %w", tokenURL, err)
		}
		return token, nil
	}(ctx)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, azURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("kafkabp.awsRackID: failed to create request from url %q: %w", azURL, err)
	}
	req.Header.Set("X-aws-ec2-metadata-token", token)

	id, err := doHTTP(req, readLimit)
	if err != nil {
		err = fmt.Errorf("kafkabp.awsRackID: failed to get AWS availability zone from url %q: %w", azURL, err)
	}
	return id, err
})

// AWSAvailabilityZoneRackID is a RackIDFunc implementation that returns AWS
// availability zone as the rack id.
//
// It also caches the result globally, so if you have more than one
// AWSAvailabilityZoneRackID in your process only the first one actually makes
// the HTTP request, for example:
//
//	consumer1 := kafkabp.NewConsumer(kafkabp.ConsumerConfig{
//	    RackID: kafkabp.AWSAvailabilityZoneRackID,
//	    Topic:  "topic1",
//	    // other configs
//	})
//	consumer2 := kafkabp.NewConsumer(kafkabp.ConsumerConfig{
//	    RackID: kafkabp.AWSAvailabilityZoneRackID,
//	    Topic:  "topic2",
//	    // other configs
//	})
//
// It uses AWS instance metadata HTTP API with 1second overall timeout and 1024
// HTTP response read limits..
//
// If there was an error retrieving rack id through AWS instance metadata API,
// the same error will be logged at slog's warning level every time
// AWSAvailabilityZoneRackID is called.
func AWSAvailabilityZoneRackID() string {
	id, err := awsRackID()
	if err != nil {
		awsRackFailure.Inc()
		slog.Warn("Failed to get AWS availability zone as rack id", "err", err)
		return ""
	}
	return id
}
