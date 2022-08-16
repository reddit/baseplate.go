// Package maxprocs optimizes GOMAXPROCS for Infrared environments.
// It attempts to balance application performance with cluster-level resource utilization.
// Applications may tune GOMAXPROCS as necessary (see Set).
// The defaults in this package are subject to change.
//
// NOTE: this is manually copied from internal baseplate/v2/internal/maxprocs/maxprocs.go
package maxprocs

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	ubermaxprocs "go.uber.org/automaxprocs/maxprocs"

	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
)

type floatEnv struct {
	key   string
	gauge *prometheus.GaugeVec

	present bool
	raw     string
	val     float64
}

const (
	// The default scale applies when request is set but not scale (or scale is
	// invalid). It defaults to some "oversubscription" to take advantage of
	// excess Kubernetes node capacity when available.
	//
	// Extremely performance-sensitive services should set scale to 1.0 to
	// ensure consistent performance.
	//
	// This value was NOT chosen scientifically and IS subject to change.
	defaultCPURequestScale = 1.5

	setByGOMAXPROCS   = "gomaxprocs"
	setByRequest      = "cpu_request"
	setByAutomaxprocs = "automaxprocs"
)

var (
	mEnvGOMAXPROCS = promauto.With(internalv2compat.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "baseplate_go_env_gomaxprocs",
		Help: "Value of the GOMAXPROCS environment variable at startup. 0 if not a number",
	}, []string{"status"})
	mEnvCPURequest = promauto.With(internalv2compat.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "baseplate_go_env_baseplate_cpu_request",
		Help: "Value of the BASEPLATE_CPU_REQUEST environment variable at startup. 0 if not a number",
	}, []string{"status"})
	mEnvCPURequestScale = promauto.With(internalv2compat.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "baseplate_go_env_baseplate_cpu_request_scale",
		Help: "Value of the BASEPLATE_CPU_REQUEST_SCALE environment variable at startup. 0 if not a number",
	}, []string{"status"})

	initialGOMAXPROCS = promauto.With(internalv2compat.GlobalRegistry).NewGaugeVec(prometheus.GaugeOpts{
		Name: "baseplate_go_initial_gomaxprocs",
		Help: "Resolved value of GOMAXPROCS at startup",
	}, []string{"set_by"})

	_ = promauto.With(internalv2compat.GlobalRegistry).NewGaugeFunc(prometheus.GaugeOpts{
		Name: "baseplate_go_current_gomaxprocs",
		Help: "Current value of GOMAXPROCS",
	}, currentGOMAXPROCS)

	// overridable for tests
	setWithAutomaxprocs = func() {
		ubermaxprocs.Set(
			ubermaxprocs.Min(2),
			ubermaxprocs.Logger(func(s string, i ...interface{}) {
				fmt.Fprintf(os.Stderr, s, i...) // contains context
			}),
		)
	}
)

// Set configures the runtime's GOMAXPROCS using the following heuristic:
//
//  1. If $GOMAXPROCS is set, Set relinquishes control to the Go runtime.
//     This should cause the runtime to respect this value directly.
//  2. If $BASEPLATE_CPU_REQUEST is unset/invalid, Set relinquishes control to automaxprocs, minimum 2.
//     See https://pkg.go.dev/go.uber.org/automaxprocs for specific behavior.
//  3. Otherwise, $BASEPLATE_CPU_REQUEST is multiplied by $BASEPLATE_CPU_REQUEST_SCALE
//     (or defaultCPURequestScale) to compute the new GOMAXPROCS, minimum 2.
//
// Set also exports several metrics to facilitate further tuning/analysis.
func Set() {
	envGOMAXPROCS := &floatEnv{key: "GOMAXPROCS", gauge: mEnvGOMAXPROCS}
	envCPURequest := &floatEnv{key: "BASEPLATE_CPU_REQUEST", gauge: mEnvCPURequest}
	envCPURequestScale := &floatEnv{key: "BASEPLATE_CPU_REQUEST_SCALE", gauge: mEnvCPURequestScale}

	for _, env := range []*floatEnv{envGOMAXPROCS, envCPURequest, envCPURequestScale} {
		env.raw, env.present = os.LookupEnv(env.key)

		var err error
		env.val, err = strconv.ParseFloat(env.raw, 64)

		var status string
		switch {
		case env.present && err == nil:
			status = "present"
		case env.present && err != nil:
			status = "not_a_number"
		default:
			status = "absent"
		}

		env.gauge.WithLabelValues(status).Set(env.val)
	}

	setBy := setByGOMAXPROCS
	defer func() {
		initialGOMAXPROCS.WithLabelValues(setBy).Set(currentGOMAXPROCS())
	}()

	if envGOMAXPROCS.present {
		return // let Go runtime handle it
	}

	if !envCPURequest.present {
		setBy = setByAutomaxprocs
		setWithAutomaxprocs()
		return
	}

	if envCPURequest.val <= 0 {
		// This should always be valid positive float in infrared-deployed applications.
		fmt.Fprintf(os.Stderr, "maxprocs: $BASEPLATE_CPU_REQUEST=%q, want positive float. Falling back to automaxprocs", envCPURequest.raw)
		setBy = setByAutomaxprocs
		setWithAutomaxprocs()
		return
	}

	scale := defaultCPURequestScale
	if envCPURequestScale.val > 0 {
		scale = envCPURequestScale.val
	} else if envCPURequestScale.present {
		fmt.Fprintf(os.Stderr, "maxprocs: $BASEPLATE_CPU_REQUEST_SCALE=%q, want positive float. Falling back to default of %g", envCPURequestScale.raw, scale)
	}

	setBy = setByRequest
	runtime.GOMAXPROCS(int(
		math.Max(
			2, // to ensure some minimal parallelism
			math.Ceil(envCPURequest.val*scale),
		),
	))
}

func currentGOMAXPROCS() float64 {
	return float64(runtime.GOMAXPROCS(0))
}
