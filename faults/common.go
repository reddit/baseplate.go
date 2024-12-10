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

// SleepFn is the function type to sleep for the given duration. It is only
// exposed for testing purposes.
type SleepFn func(d time.Duration)

func getPercentage(percentageHeader string, GetHeaderFn func(key string) string) (int, error) {
	percentageStr := GetHeaderFn(percentageHeader)
	if percentageStr == "" {
		return 100, nil
	}
	percentage, err := strconv.Atoi(percentageStr)
	if err != nil {
		return 0, fmt.Errorf("provided percentage %q is not a valid integer: %w", percentageStr, err)
	}
	if percentage < 0 || percentage > 100 {
		return 0, fmt.Errorf("provided percentage %q is outside the valid range of [0-100]", percentage)
	}
	return percentage, nil
}

type InjectFaultParams[T any] struct {
	CallerName string

	Address, Method            string
	AbortCodeMin, AbortCodeMax int

	GetHeaderFn GetHeaderFn
	ResumeFn    ResumeFn[T]
	ResponseFn  ResponseFn[T]

	// Exposed for tests.
	RandInt *int
	SleepFn *SleepFn
}

func InjectFault[T any](params InjectFaultParams[T]) (T, error) {
	serverAddress := params.GetHeaderFn(FaultServerAddressHeader)

	// The short address should just be <service>.<namespace>, without the
	// local cluster suffix or port. Non-cluster-local addresses are not
	// shortened.
	shortAddress := params.Address
	if i := strings.Index(params.Address, ".svc.cluster.local"); i != -1 {
		shortAddress = params.Address[:i]
	}
	if serverAddress == "" || serverAddress != shortAddress {
		return params.ResumeFn()
	}

	serverMethod := params.GetHeaderFn(FaultServerMethodHeader)
	if serverMethod != "" && serverMethod != params.Method {
		return params.ResumeFn()
	}

	var randInt int
	if params.RandInt != nil {
		randInt = *params.RandInt
	} else {
		randInt = rand.IntN(100)
	}

	delayMs := params.GetHeaderFn(FaultDelayMsHeader)
	if delayMs != "" {
		percentage, err := getPercentage(FaultDelayPercentageHeader, params.GetHeaderFn)
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
		if params.SleepFn != nil {
			sleepFn = *params.SleepFn
		}
		sleepFn(time.Duration(delay) * time.Millisecond)
	}

	abortCode := params.GetHeaderFn(FaultAbortCodeHeader)
	if abortCode != "" {
		percentage, err := getPercentage(FaultAbortPercentageHeader, params.GetHeaderFn)
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
