package httpbp

import (
	"errors"
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
func ServeAdmin(healthCheck HandlerFunc) {
	admin.Mux.Handle("/health", handler{handle: healthCheck})
	if err := admin.Serve(); errors.Is(err, http.ErrServerClosed) {
		log.Info("httpbp: server closed")
	} else {
		log.Panicw("httpbp: admin serving failed", "err", err)
	}
}
