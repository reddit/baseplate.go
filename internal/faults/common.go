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
)

// GetHeaderFn is the function type to return the value of a protocol-specific
// header with the given key.
type GetHeaderFn func(key string) string

// ResumeFn is the function type to continue processing the protocol-specific
// request without injecting a fault.
type ResumeFn[T any] func() (T, error)

// ResponseFn is the function type to inject a protocol-specific fault with the
// given code and message.
type ResponseFn[T any] func(code int, message string) (T, error)

// sleepFn is the function type to sleep for the given duration. Only used in
// tests.
type sleepFn func(ctx context.Context, d time.Duration) error

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

func selected(randInt *int, percentage int) bool {
	if randInt != nil {
		return *randInt < percentage
	}
	// Use a different random integer per feature as per
	// https://github.com/grpc/proposal/blob/master/A33-Fault-Injection.md#evaluate-possibility-fraction.
	return rand.IntN(100) < percentage
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	select {
	case <-t.C:
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	}
	return nil
}

type InjectFaultParams[T any] struct {
	Context    context.Context
	CallerName string

	Address, Method            string
	AbortCodeMin, AbortCodeMax int

	GetHeaderFn GetHeaderFn
	ResumeFn    ResumeFn[T]
	ResponseFn  ResponseFn[T]

	randInt *int
	sleepFn *sleepFn
}

func InjectFault[T any](params InjectFaultParams[T]) (T, error) {
	slog.Info(fmt.Sprintf("Starting InjectFault with the following parameters:\n- CallerName: %s\n- Address: %s\n- Method: %s\n- AbortCodeMin: %d\n- AbortCodeMax: %d\n", params.CallerName, params.Address, params.Method, params.AbortCodeMin, params.AbortCodeMax))

	faultHeaderAddress := params.GetHeaderFn(FaultServerAddressHeader)
	requestAddress := getCanonicalAddress(params.Address)
	if faultHeaderAddress == "" || faultHeaderAddress != requestAddress {
		slog.Info(fmt.Sprintf("Skipping InjectFault as the faultHeaderAddress is %q and the requestAddress is %q", faultHeaderAddress, requestAddress))
		return params.ResumeFn()
	}

	serverMethod := params.GetHeaderFn(FaultServerMethodHeader)
	if serverMethod != "" && serverMethod != params.Method {
		slog.Info(fmt.Sprintf("Skipping InjectFault as the serverMethod is %q and the params.Method is %q", serverMethod, params.Method))
		return params.ResumeFn()
	}

	delayMs := params.GetHeaderFn(FaultDelayMsHeader)
	if delayMs != "" {
		percentage, err := parsePercentage(params.GetHeaderFn(FaultDelayPercentageHeader))
		if err != nil {
			slog.Warn(fmt.Sprintf("%s: %v", params.CallerName, err))
			return params.ResumeFn()
		}

		if selected(params.randInt, percentage) {
			delay, err := strconv.Atoi(delayMs)
			if err != nil {
				slog.Warn(fmt.Sprintf("%s: provided delay %q is not a valid integer", params.CallerName, delayMs))
				return params.ResumeFn()
			}

			sleepFn := sleep
			if params.sleepFn != nil {
				sleepFn = *params.sleepFn
			}
			if err := sleepFn(params.Context, time.Duration(delay)*time.Millisecond); err != nil {
				slog.Warn(fmt.Sprintf("%s: error when delaying request: %v", params.CallerName, err))
				return params.ResumeFn()
			}
		}
	}

	abortCode := params.GetHeaderFn(FaultAbortCodeHeader)
	if abortCode != "" {
		percentage, err := parsePercentage(params.GetHeaderFn(FaultAbortPercentageHeader))
		if err != nil {
			slog.Warn(fmt.Sprintf("%s: %v", params.CallerName, err))
			return params.ResumeFn()
		}

		if selected(params.randInt, percentage) {
			code, err := strconv.Atoi(abortCode)
			if err != nil {
				slog.Warn(fmt.Sprintf("%s: provided abort code %q is not a valid integer", params.CallerName, abortCode))
				return params.ResumeFn()
			}
			if code < params.AbortCodeMin || code > params.AbortCodeMax {
				slog.Warn(fmt.Sprintf("%s: provided abort code \"%d\" is outside of the valid range", params.CallerName, code))
				return params.ResumeFn()
			}
			abortMessage := params.GetHeaderFn(FaultAbortMessageHeader)
			slog.Info(fmt.Sprintf("Injecting fault with code %d and message %q", code, abortMessage))
			return params.ResponseFn(code, abortMessage)
		}
	}

	slog.Info("No abort fault injected")
	return params.ResumeFn()
}
