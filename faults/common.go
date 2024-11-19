package faults

import (
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/rand"
)

func getShortenedAddress(address string) string {
	parts := strings.Split(address, ".")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:2], ".")
}

func InjectFault(address, method string, abortCodeMin, abortCodeMax int, getHeaderFn func(key string) string, resumeFn func() (interface{}, error), responseFn func(code int, message string) (interface{}, error)) (interface{}, error) {
	serverAddress := getHeaderFn(FaultServerAddressHeader)
	if serverAddress == "" || serverAddress != getShortenedAddress(address) {
		return resumeFn()
	}

	serverMethod := getHeaderFn(FaultServerMethodHeader)
	if serverMethod != "" && serverMethod != method {
		return resumeFn()
	}

	delayMs := getHeaderFn(FaultDelayMsHeader)
	if delayMs != "" {
		delayPercentage := getHeaderFn(FaultDelayPercentageHeader)
		if delayPercentage != "" {
			percentage, err := strconv.Atoi(delayPercentage)
			if err != nil {
				// log "provided delay percentage is not a valid integer"
				return resumeFn()
			}
			if percentage < 0 || percentage > 100 {
				// log "provided delay percentage is outside the valid range of [0-100]"
				return resumeFn()
			}
			if percentage == 0 || (percentage != 100 && rand.Intn(100) >= percentage) {
				return resumeFn()
			}
		}

		delay, err := strconv.Atoi(delayMs)
		if err != nil {
			// log "provided delay is not a valid integer"
			return resumeFn()
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	abortCode := getHeaderFn(FaultAbortCodeHeader)
	if abortCode != "" {
		abortPercentage := getHeaderFn(FaultAbortPercentageHeader)
		if abortPercentage != "" {
			percentage, err := strconv.Atoi(abortPercentage)
			if err != nil {
				// log "provided abort percentage is not a valid integer"
				return resumeFn()
			}
			if percentage < 0 || percentage > 100 {
				// log "provided abort percentage is outside the valid range of [0-100]"
				return resumeFn()
			}
			if percentage != 100 && rand.Intn(100) >= percentage {
				return resumeFn()
			}
		}

		code, err := strconv.Atoi(abortCode)
		if err != nil {
			// log "provided abort code is not a valid integer"
			return resumeFn()
		}
		if code < abortCodeMin || code > abortCodeMax {
			// log "provided abort code is outside of the valid range"
			return resumeFn()
		}
		abortMessage := getHeaderFn(FaultAbortMessageHeader)
		return responseFn(code, abortMessage)
	}

	return resumeFn()
}
