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
	// Lookup returns the value of a protocol-specific header with the
	// given key.
	Lookup(ctx context.Context, key string) (string, error)
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

func parsePercentage(percentage string) (int, error) {
	if percentage == "" {
		return 100, nil
	}
	intPercentage, err := strconv.Atoi(percentage)
	if err != nil {
		return 0, fmt.Errorf("provided percentage %q is not a valid integer: %w", percentage, err)
	}
	if intPercentage < 0 || intPercentage > 100 {
		return 0, fmt.Errorf("provided percentage \"%d\" is outside the valid range of [0-100]", intPercentage)
	}
	return intPercentage, nil
}

// Injector contains the data common across all requests needed to inject
// faults on outgoing requests.
type Injector[T any] struct {
	clientName   string
	callerName   string
	abortCodeMin int
	abortCodeMax int

	defaultAbort Abort[T]

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
		slog.With("caller", i.callerName).InfoContext(ctx, fmt.Sprintf(format, args...))
	}
	warnf := func(format string, args ...interface{}) {
		slog.With("caller", i.callerName).WarnContext(ctx, fmt.Sprintf(format, args...))
	}

	faultHeaderAddress, err := headers.Lookup(ctx, FaultServerAddressHeader)
	if err != nil {
		infof("error looking up header %q: %v", FaultServerAddressHeader, err)
		totalReqsCounter(true, false).Inc()
		return resume()
	}
	requestAddress := getCanonicalAddress(address)
	if faultHeaderAddress == "" || faultHeaderAddress != requestAddress {
		totalReqsCounter(true, false).Inc()
		return resume()
	}

	serverMethod, err := headers.Lookup(ctx, FaultServerMethodHeader)
	if err != nil {
		infof("error looking up header %q: %v", FaultServerMethodHeader, err)
		totalReqsCounter(true, false).Inc()
		return resume()
	}
	if serverMethod != "" && serverMethod != method {
		totalReqsCounter(true, false).Inc()
		return resume()
	}

	delayMs, err := headers.Lookup(ctx, FaultDelayMsHeader)
	if err != nil {
		infof("error looking up header %q: %v", FaultDelayMsHeader, err)
	}
	if delayMs != "" {
		percentageHeader, err := headers.Lookup(ctx, FaultDelayPercentageHeader)
		if err != nil {
			infof("error looking up header %q: %v", FaultDelayPercentageHeader, err)
		}
		percentage, err := parsePercentage(percentageHeader)
		if err != nil {
			warnf("error parsing percentage header %q: %v", FaultDelayPercentageHeader, err)
			totalReqsCounter(false, false).Inc()
			return resume()
		}

		if i.selected(percentage) {
			delay, err := strconv.Atoi(delayMs)
			if err != nil {
				warnf("unable to convert provided delay %q to integer: %v", delayMs, err)
				totalReqsCounter(false, false).Inc()
				return resume()
			}

			if err := i.sleep(ctx, time.Duration(delay)*time.Millisecond); err != nil {
				warnf("error when delaying request: %v", err)
				totalReqsCounter(false, false).Inc()
				return resume()
			}
			delayed = true
		}
	}

	abortCode, err := headers.Lookup(ctx, FaultAbortCodeHeader)
	if err != nil {
		infof("error looking up header %q: %v", FaultAbortCodeHeader, err)
	}
	if abortCode != "" {
		percentageHeader, err := headers.Lookup(ctx, FaultAbortPercentageHeader)
		if err != nil {
			infof("error looking up header %q: %v", FaultAbortPercentageHeader, err)
		}
		percentage, err := parsePercentage(percentageHeader)
		if err != nil {
			warnf("error parsing percentage header %q: %v", FaultAbortPercentageHeader, err)
			totalReqsCounter(false, false).Inc()
			return resume()
		}

		if i.selected(percentage) {
			code, err := strconv.Atoi(abortCode)
			if err != nil {
				warnf("unable to convert provided abort %q to integer: %v", abortCode, err)
				totalReqsCounter(false, false).Inc()
				return resume()
			}
			if code < i.abortCodeMin || code > i.abortCodeMax {
				warnf("provided abort code \"%d\" is outside of the valid range [%d-%d]", code, i.abortCodeMin, i.abortCodeMax)
				totalReqsCounter(false, false).Inc()
				return resume()
			}

			abortMessage, err := headers.Lookup(ctx, FaultAbortMessageHeader)
			if err != nil {
				warnf("error looking up header %q: %v", FaultAbortMessageHeader, err)
				totalReqsCounter(false, false).Inc()
				return resume()
			}

			totalReqsCounter(true, true).Inc()
			return abort(code, abortMessage)
		}
	}

	totalReqsCounter(true, false).Inc()
	return resume()
}

// Inject injects a fault using the Injector default fault function on the
// outgoing request if it matches the header configuration.
func (i *Injector[T]) Inject(ctx context.Context, address, method string, headers Headers, resume Resume[T]) (T, error) {
	return i.InjectWithAbortOverride(ctx, address, method, headers, resume, i.defaultAbort)
}
