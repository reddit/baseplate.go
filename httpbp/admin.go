package httpbp

import (
	"net/http"

	"github.com/reddit/baseplate.go/internal/admin"
	"github.com/reddit/baseplate.go/log"
)

// ServeAdmin starts a blocking HTTP server for internal functions:
//    health        - serve /health for health checking
//    metrics       - serve /metrics for prometheus
//    profiling     - serve /debug/pprof for profiling, ref: https://pkg.go.dev/net/http/pprof
//
// Default server address is admin.Addr.
//
// This function blocks, so it should be run as its own goroutine.
func ServeAdmin(healthCheck http.HandlerFunc) {
	admin.Mux.HandleFunc("/health", healthCheck)
	log.Warnw("httpbp: admin serving exited", "err", admin.Serve())
}
