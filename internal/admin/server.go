package admin

import (
	"net/http"
	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const DefaultAdminAddr = ":6060"

// DefaultAdminServeMux is the default ServeMux to be used for admin servers in packages like httpbp, thriftbp, etc.
// DefaultAdminServeMux configures the following routes:
//    metrics       - serve /metrics for prometheus
//    profiling     - serve /debug/pprof for profiling, ref: https://pkg.go.dev/net/http/pprof
var DefaultAdminServeMux = http.NewServeMux()

func init() {
	// The debug/pprof endpoints follow the pattern from the init function in net/http/pprof package.
	DefaultAdminServeMux.HandleFunc("/debug/pprof/", pprof.Index)
	DefaultAdminServeMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	DefaultAdminServeMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	DefaultAdminServeMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	DefaultAdminServeMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	DefaultAdminServeMux.Handle("/metrics", promhttp.Handler())
}
