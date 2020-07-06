package httpbp

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
)

// HealthCheckProbeQuery is the name of HTTP query defined in Baseplate spec.
const HealthCheckProbeQuery = "type"

// GetHealthCheckProbe parses the health check probe from the request.
//
// Unrecognized string probes will fallback to READINESS.
// When that happens a non-nil error will also be returned.
// If a probe is not specified in the request,
// this function will return READINESS with nil error.
//
// This function only supports the probes known to this version of Baseplate.go
// for the string version of probes passed into the request.
// If in the future a new probe is added to baseplate.thrift,
// you would need to update Baseplate.go library,
// otherwise it would fallback to READINESS.
// Currently the supported probes are:
//
// - READINESS
//
// - LIVENESS
//
// - STARTUP
//
// If the probe specified in the request is the int value,
// this function just blindly return the parsed int value,
// even if it's not one of the defined enum values known to this version of
// Baseplate.go.
//
// Note that because of go's type system,
// to make this function more useful the returned probe value is casted back to
// int64 from the thrift defined enum type.
func GetHealthCheckProbe(query url.Values) (int64, error) {
	code := query.Get(HealthCheckProbeQuery)
	// Fallback to READINESS when it's not specified.
	if code == "" {
		return int64(baseplate.IsHealthyProbe_READINESS), nil
	}
	// Handle the int types.
	if probe, err := strconv.ParseInt(code, 10, 64); err == nil {
		return probe, nil
	}
	// Handle the string types.
	if probe, err := baseplate.IsHealthyProbeFromString(strings.ToUpper(code)); err == nil {
		return int64(probe), nil
	}
	// Fallback to READINESS, with an error.
	return int64(baseplate.IsHealthyProbe_READINESS), fmt.Errorf(
		"httpbp.GetHealthCheckProbe: unrecognized probe type %q, fallback to READINESS",
		code,
	)
}
