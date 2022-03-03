package thriftbp

import (
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
	log.Warnw("thriftbp: admin serving exited", "err", admin.Serve())
}
