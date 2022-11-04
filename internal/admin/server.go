package admin

import (
	"net/http"
	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/reddit/baseplate.go/log"
)

const Addr = ":6060"

// Mux is the default ServeMux to be used for admin servers in packages like httpbp, thriftbp, etc.
// Mux configures the following routes:
//
//	metrics       - serve /metrics for prometheus
//	profiling     - serve /debug/pprof for profiling, ref: https://pkg.go.dev/net/http/pprof
var Mux = http.NewServeMux()

var baseplateGoCollectors = collectors.WithGoCollectorRuntimeMetrics(
	collectors.MetricsScheduler,
)

func init() {
	// The debug/pprof endpoints follow the pattern from the init function in net/http/pprof package.
	// ref: https://cs.opensource.google/go/go/+/refs/tags/go1.17.7:src/net/http/pprof/pprof.go;l=80
	Mux.HandleFunc("/debug/pprof/", pprof.Index)
	Mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	Mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	Mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	Mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	Mux.Handle("/metrics", promhttp.Handler())

	// Unregister the default GoCollector, and reregister with baseplate defaults
	if prometheus.Unregister(collectors.NewGoCollector()) {
		// Only register a new collector if we unregistered one to avoid double-reregistration
		prometheus.MustRegister(collectors.NewGoCollector(baseplateGoCollectors))
	}
}

func Serve() error {
	log.Infof("Serving admin on %s", Addr)
	return http.ListenAndServe(Addr, Mux)
}
