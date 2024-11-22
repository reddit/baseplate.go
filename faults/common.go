package faults

import (
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/rand"
)

type GetHeaderFn func(key string) string
type ResumeFn func() (interface{}, error)
type ResponseFn func(code int, message string) (interface{}, error)
type SleepFn func(d time.Duration)

type randSingleton struct {
	randInt *int
}

func (r randSingleton) getRandInt() int {
	if r.randInt == nil {
		*r.randInt = rand.Intn(100)
	}
	return *r.randInt
}

func getShortenedAddress(address string) string {
	parts := strings.Split(address, ".")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:2], ".")
}

func isSelected(percentageHeader string, GetHeaderFn func(key string) string, singleRand randSingleton) bool {
	percentageStr := GetHeaderFn(percentageHeader)
	if percentageStr == "" {
		return true
	}
	percentage, err := strconv.Atoi(percentageStr)
	if err != nil {
		// log "provided delay percentage is not a valid integer"
		return false
	}
	if percentage < 0 || percentage > 100 {
		// log "provided delay percentage is outside the valid range of [0-100]"
		return false
	}
	return singleRand.getRandInt() < percentage
}

type InjectFaultParams struct {
	Address, Method            string
	AbortCodeMin, AbortCodeMax int

	GetHeaderFn GetHeaderFn
	ResumeFn    ResumeFn
	ResponseFn  ResponseFn

	// Exposed for tests
	RandInt *int
	SleepFn *SleepFn
}

func InjectFault(params InjectFaultParams) (interface{}, error) {
	serverAddress := params.GetHeaderFn(FaultServerAddressHeader)
	if serverAddress == "" || serverAddress != getShortenedAddress(params.Address) {
		return params.ResumeFn()
	}

	serverMethod := params.GetHeaderFn(FaultServerMethodHeader)
	if serverMethod != "" && serverMethod != params.Method {
		return params.ResumeFn()
	}

	singleRand := randSingleton{
		randInt: params.RandInt,
	}

	delayMs := params.GetHeaderFn(FaultDelayMsHeader)
	if delayMs != "" {
		if !isSelected(FaultDelayPercentageHeader, params.GetHeaderFn, singleRand) {
			return params.ResumeFn()
		}

		delay, err := strconv.Atoi(delayMs)
		if err != nil {
			// log "provided delay is not a valid integer"
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
		if !isSelected(FaultAbortPercentageHeader, params.GetHeaderFn, singleRand) {
			return params.ResumeFn()
		}

		code, err := strconv.Atoi(abortCode)
		if err != nil {
			// log "provided abort code is not a valid integer"
			return params.ResumeFn()
		}
		if code < params.AbortCodeMin || code > params.AbortCodeMax {
			// log "provided abort code is outside of the valid range"
			return params.ResumeFn()
		}
		abortMessage := params.GetHeaderFn(FaultAbortMessageHeader)
		return params.ResponseFn(code, abortMessage)
	}

	return params.ResumeFn()
}
