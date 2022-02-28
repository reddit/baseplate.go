package httpbp

import (
	"net/http"

	"github.com/reddit/baseplate.go/internal/admin"
	"github.com/reddit/baseplate.go/log"
)

// AdminServerArgs contain optional configuration for the admin server.
type AdminServerArgs struct {
	// AdminAddr is a custom address for the admin server. Defaults to DefaultAdminAddr.
	AdminAddr string
	// HealthCheckFn is the HTTP handler for health check.
	HealthCheckFn http.HandlerFunc
}

// NewAdminServer returns a new admin server for internal functionality.
func NewAdminServer(args *AdminServerArgs) *AdminServer {
	adminAddr := args.AdminAddr
	if args.AdminAddr == "" {
		adminAddr = admin.DefaultAdminAddr
	}

	return &AdminServer{
		addr:          adminAddr,
		healthCheckFn: args.HealthCheckFn,
	}
}

type AdminServer struct {
	addr          string
	healthCheckFn http.HandlerFunc
}

// Serve starts a blocking HTTP server for internal functions:
//    metrics       - serve /metrics for prometheus
//    health check  - serve /health for health checking
//    profiling     - serve /debug/pprof for profiling, ref: https://pkg.go.dev/net/http/pprof
//
// Default server address is admin.DefaultAdminAddr.
//
// This method is blocking, to prevent blocking run it as a goroutine.
func (s *AdminServer) Serve() {
	if s.healthCheckFn != nil {
		admin.DefaultAdminServeMux.HandleFunc("/health", s.healthCheckFn)
	}
	log.Infof("Serving admin on %s", s.addr)
	log.Warnw("admin http serve exited", "err", http.ListenAndServe(s.addr, admin.DefaultAdminServeMux))
	log.Info("admin returnings")
}
