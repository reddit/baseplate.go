// Package faults provides common headers and client-side fault injection
// functionality.
package faults

import (
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
type sleepFn func(d time.Duration)

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

type InjectFaultParams[T any] struct {
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
	faultHeaderAddress := params.GetHeaderFn(FaultServerAddressHeader)
	requestAddress := getCanonicalAddress(params.Address)
	if faultHeaderAddress == "" || faultHeaderAddress != requestAddress {
		return params.ResumeFn()
	}

	serverMethod := params.GetHeaderFn(FaultServerMethodHeader)
	if serverMethod != "" && serverMethod != params.Method {
		return params.ResumeFn()
	}

	var randInt int
	if params.randInt != nil {
		randInt = *params.randInt
	} else {
		randInt = rand.IntN(100)
	}

	delayMs := params.GetHeaderFn(FaultDelayMsHeader)
	if delayMs != "" {
		percentage, err := parsePercentage(params.GetHeaderFn(FaultDelayPercentageHeader))
		if err != nil {
			slog.Warn(fmt.Sprintf("%s: %v", params.CallerName, err))
			return params.ResumeFn()
		}
		if randInt >= percentage {
			return params.ResumeFn()
		}

		delay, err := strconv.Atoi(delayMs)
		if err != nil {
			slog.Warn(fmt.Sprintf("%s: provided delay \"%s\" is not a valid integer", params.CallerName, delayMs))
			return params.ResumeFn()
		}

		sleepFn := time.Sleep
		if params.sleepFn != nil {
			sleepFn = *params.sleepFn
		}
		sleepFn(time.Duration(delay) * time.Millisecond)
	}

	abortCode := params.GetHeaderFn(FaultAbortCodeHeader)
	if abortCode != "" {
		percentage, err := parsePercentage(params.GetHeaderFn(FaultAbortPercentageHeader))
		if err != nil {
			slog.Warn(fmt.Sprintf("%s: %v", params.CallerName, err))
			return params.ResumeFn()
		}
		if randInt >= percentage {
			return params.ResumeFn()
		}

		code, err := strconv.Atoi(abortCode)
		if err != nil {
			slog.Warn(fmt.Sprintf("%s: provided abort code \"%s\" is not a valid integer", params.CallerName, abortCode))
			return params.ResumeFn()
		}
		if code < params.AbortCodeMin || code > params.AbortCodeMax {
			slog.Warn(fmt.Sprintf("%s: provided abort code \"%d\" is outside of the valid range", params.CallerName, code))
			return params.ResumeFn()
		}
		abortMessage := params.GetHeaderFn(FaultAbortMessageHeader)
		return params.ResponseFn(code, abortMessage)
	}

	return params.ResumeFn()
}
