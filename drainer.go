package baseplate

import (
	"context"
	"io"
	"sync/atomic"
)

// HealthChecker defines an interface to report healthy status.
type HealthChecker interface {
	IsHealthy(ctx context.Context) bool
}

// HealthCheckCloser is the combination of Healthyer and io.Closer.
type HealthCheckCloser interface {
	HealthChecker
	io.Closer
}

type drainer struct {
	closed int64
}

func (d *drainer) IsHealthy(_ context.Context) bool {
	return atomic.LoadInt64(&d.closed) == 0
}

func (d *drainer) Close() error {
	atomic.StoreInt64(&d.closed, 1)
	return nil
}

// Drainer creates a HealthCheckCloser implementation that can be used to drain
// service during graceful shutdown.
//
// The HealthCheckCloser returned would start to report healthy once created,
// and start to report unhealthy as soon as its Close is called.
//
// Please refer to the example on how to use this feature in your service.
// Basically you should create a Drainer in your main function,
// add it to the PreShutdown closers list in baseplate.Serve,
// and then fail your readiness health check when drainer is reporting unhealty.
//
// Its Close function would never return an error.
// It's also OK to call Close function multiple times.
// Calls after the first one are essentially no-ops.
func Drainer() HealthCheckCloser {
	return new(drainer)
}
