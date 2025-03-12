// Package faults provides common headers and client-side fault injection
// functionality.
package faults

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/time/rate"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
)

const (
	promNamespace      = "faultbp"
	clientNameLabel    = "fault_client_name"
	serviceLabel       = "fault_service"
	methodLabel        = "fault_method"
	protocolLabel      = "fault_protocol"
	successLabel       = "fault_success"
	delayInjectedLabel = "fault_injected_delay"
	abortInjectedLabel = "fault_injected_abort"
)

var (
	totalRequests = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "faultbp_fault_requests_total",
		Help: "Total count of requests seen by the fault injection middleware.",
	}, []string{
		clientNameLabel,
		serviceLabel,
		methodLabel,
		protocolLabel,
		successLabel,
		delayInjectedLabel,
		abortInjectedLabel,
	})
)

// Headers is an interface to be implemented by the caller to allow
// protocol-specific header lookup. Using an interface here rather than a
// function type avoids any potential closure requirements of a function.
type Headers interface {
	// LookupValues returns the values of a protocol-specific header with
	// the given key.
	LookupValues(ctx context.Context, key string) ([]string, error)
}

// Resume is the function type to continue processing the protocol-specific
// request without injecting a fault.
type Resume[T any] func() (T, error)

// Abort is the function type to inject a protocol-specific fault with the
// given code and message.
type Abort[T any] func(code int, message string) (T, error)

// The canonical address for a cluster-local address is <service>.<namespace>,
// without the local cluster suffix or port. The canonical address for a
// non-cluster-local address is the full original address without the port.
func getCanonicalAddress(serverAddress string) string {
	// Cluster-local address.
	if i := strings.Index(serverAddress, ".svc.cluster.local"); i != -1 {
		return serverAddress[:i]
	}
	// External host:port address.
	if i := strings.LastIndex(serverAddress, ":"); i != -1 {
		port := serverAddress[i+1:]
		// Verify this is actually a port number.
		if port != "" && port[0] >= '0' && port[0] <= '9' {
			return serverAddress[:i]
		}
	}
	// Other address, i.e. unix domain socket.
	return serverAddress
}

// Injector contains the data common across all requests needed to inject
// faults on outgoing requests.
type Injector[T any] struct {
	clientName   string
	callerName   string
	abortCodeMin int
	abortCodeMax int

	defaultAbort Abort[T]

	chatty *rate.Limiter // Rate limiter for logs.

	selected func(int) bool
	sleep    func(context.Context, time.Duration) error
}

// WithDefaultAbort is an option to set the default abort function for the
// Injector.
func WithDefaultAbort[T any](fn Abort[T]) func(*Injector[T]) {
	return func(i *Injector[T]) {
		i.defaultAbort = fn
	}
}

func defaultSelected(percentage int) bool {
	// Use a different random integer per feature as per
	// https://github.com/grpc/proposal/blob/master/A33-Fault-Injection.md#evaluate-possibility-fraction.
	return rand.IntN(100) < percentage
}

func defaultSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	select {
	case <-t.C:
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	}
	return nil
}

// NewInjector creates a new Injector with the provided parameters.
func NewInjector[T any](clientName, callerName string, abortCodeMin, abortCodeMax int, option ...func(*Injector[T])) *Injector[T] {
	i := &Injector[T]{
		clientName:   clientName,
		callerName:   callerName,
		abortCodeMin: abortCodeMin,
		abortCodeMax: abortCodeMax,
		chatty:       rate.NewLimiter(rate.Every(1*time.Minute), 1),
		selected:     defaultSelected,
		sleep:        defaultSleep,
	}
	for _, o := range option {
		o(i)
	}
	return i
}

// InjectWithAbortOverride injects a fault using the provided fault function
// on the outgoing request if it matches the header configuration.
func (i *Injector[T]) InjectWithAbortOverride(ctx context.Context, address, method string, headers Headers, resume Resume[T], abort Abort[T]) (T, error) {
	delayed := false
	totalReqsCounter := func(success, aborted bool) prometheus.Counter {
		return totalRequests.WithLabelValues(
			i.clientName,
			address,
			method,
			i.callerName,
			strconv.FormatBool(success),
			strconv.FormatBool(delayed),
			strconv.FormatBool(aborted),
		)
	}

	infof := func(format string, args ...interface{}) {
		if i.chatty == nil || i.chatty.Allow() {
			slog.With("caller", i.callerName).InfoContext(ctx, fmt.Sprintf(format, args...))
		}
	}
	warnf := func(format string, args ...interface{}) {
		if i.chatty == nil || i.chatty.Allow() {
			slog.With("caller", i.callerName).WarnContext(ctx, fmt.Sprintf(format, args...))
		}
	}

	faultHeaderValues, err := headers.LookupValues(ctx, FaultHeader)
	if err != nil {
		infof("error looking up the values of header %q: %v", FaultHeader, err)
		totalReqsCounter(true, false).Inc()
		return resume()
	}

	faultConfiguration, err := parseMatchingFaultConfiguration(faultHeaderValues, getCanonicalAddress(address), method, i.abortCodeMin, i.abortCodeMax)
	if err != nil {
		warnf("error parsing fault header %q: %v", FaultHeader, err)

		if faultConfiguration == nil {
			totalReqsCounter(false, false).Inc()
			return resume()
		}
	}
	if faultConfiguration == nil {
		totalReqsCounter(true, false).Inc()
		return resume()
	}

	if faultConfiguration.Delay > 0 && i.selected(faultConfiguration.DelayPercentage) {
		if err := i.sleep(ctx, faultConfiguration.Delay); err != nil {
			warnf("error when delaying request: %v", err)
			totalReqsCounter(false, false).Inc()
			return resume()
		}
		delayed = true
	}

	if faultConfiguration.AbortCode != -1 && i.selected(faultConfiguration.AbortPercentage) {
		totalReqsCounter(true, true).Inc()
		return abort(faultConfiguration.AbortCode, faultConfiguration.AbortMessage)
	}

	totalReqsCounter(true, false).Inc()
	return resume()
}

// Inject injects a fault using the Injector default fault function on the
// outgoing request if it matches the header configuration.
func (i *Injector[T]) Inject(ctx context.Context, address, method string, headers Headers, resume Resume[T]) (T, error) {
	return i.InjectWithAbortOverride(ctx, address, method, headers, resume, i.defaultAbort)
}
