package thriftbp

import (
	"errors"
	"net/http"

	"github.com/reddit/baseplate.go/internal/admin"
	"github.com/reddit/baseplate.go/log"
)

// ServeAdmin starts a blocking HTTP server for internal functions:
//    metrics       - serve /metrics for prometheus
//    profiling     - serve /debug/pprof for profiling, ref: https://pkg.go.dev/net/http/pprof
//
// Default server address is admin.Addr.
//
// This function blocks, so it should be run as its own goroutine.
func ServeAdmin() {
	if err := admin.Serve(); errors.Is(err, http.ErrServerClosed) {
		log.Info("thriftbp: server closed")
	} else {
		log.Panicw("thriftbp: admin serving exited", "err", err)
	}
}
