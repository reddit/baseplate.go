package admin

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/reddit/baseplate.go/log"
)

const (
	DefaultAdminAddr = ":6060"
)

// ServerArgs contain optional configuration for the admin server.
type ServerArgs struct {
	// AdminAddr is a custom address for the admin server. Defaults to port 6060 on localhost if not set.
	AdminAddr string
	// HealthCheckFn is the HTTP handler for health check.
	HealthCheckFn func(w http.ResponseWriter, r *http.Request)
}

// NewServer returns a new admin server for internal functionality.
func NewServer(args *ServerArgs) *Server {
	adminAddr := args.AdminAddr
	if args.AdminAddr == "" {
		adminAddr = DefaultAdminAddr
	}

	return &Server{
		adminAddr:     adminAddr,
		healthCheckFn: args.HealthCheckFn,
	}
}

type Server struct {
	adminAddr     string
	healthCheckFn func(w http.ResponseWriter, r *http.Request)
}

// Serve starts an HTTP server for internal functions:
//    metrics       - serve /metrics for prometheus
//    health check  - serve /health for health checking
//    pprof         - https://blog.golang.org/pprof
// Default server address is http://localhost:6060.
func (s *Server) Serve() {
	go func() {
		if s.healthCheckFn != nil {
			http.HandleFunc("/health", s.healthCheckFn)
		}
		http.Handle("/metrics", promhttp.Handler())
		log.Infof("Serving admin on %s", s.adminAddr)
		log.Warnw("admin http serve exited", "err", http.ListenAndServe(s.adminAddr, nil))
	}()
}
