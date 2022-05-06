package runtimebp

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/reddit/baseplate.go"
)

// ShutdownHandler is the callback type used in HandleSignals.
type ShutdownHandler func(signal os.Signal)

var defaultSignals = []os.Signal{
	// For ^C
	os.Interrupt,
	// Ref: https://kubernetes.io/docs/concepts/workloads/pods/pod/#termination-of-pods
	syscall.SIGTERM,
}

// State is an enum representing the runtime state of the baseplate server
type State int

const (
	StateUnknown State = iota
	StateRunning
	StateShuttingDown
)

// drainer is used to track the server state
//
// This drainer is updated by HandleShutdown before invoking the
// user supplied ShutdownHandler
var drainer = baseplate.Drainer()

// ServerState returns the current runtimebp.State of the application
func ServerState() State {
	if drainer.IsHealthy(context.Background()) {
		return StateRunning
	}

	return StateShuttingDown
}

// HandleShutdown register a handler to do cleanups for a graceful shutdown.
//
// This function blocks until the ctx passed in is cancelled,
// or a signal happens, whichever comes first.
// So it should usually be started in its own goroutine.
//
// Baseplate services should use baseplate.Serve which will manage this for you
// rather than using HandleShutdown directly.
//
// SIGTERM, as specified in
// https://kubernetes.io/docs/concepts/workloads/pods/pod/#termination-of-pods,
// and os.Interrupt as for handling ^C in command line,
// are always registered in this function and there's no need to pass them in
// (but passing them in won't cause any harm),
// the signals vararg is for any additional signals you wish to handle.
func HandleShutdown(ctx context.Context, handler ShutdownHandler, signals ...os.Signal) {
	sig := make([]os.Signal, 0, len(defaultSignals)+len(signals))
	sig = append(sig, defaultSignals...)
	sig = append(sig, signals...)
	c := make(chan os.Signal, 1)
	signal.Notify(
		c,
		sig...,
	)
	select {
	case signal := <-c:
		drainer.Close()
		handler(signal)
	case <-ctx.Done():
		drainer.Close()
		// do nothing, just unblock the select block so it will return after it.
	}
}
