package faults

import (
	"strconv"
	"strings"
	"time"

	"github.com/reddit/baseplate.go/faults"
	"golang.org/x/exp/rand"
)

func getShortenedAddress(address string) string {
	parts := strings.Split(address, ".")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:2], ".")
}

func InjectFault(address, method string, getHeaderFn func(key string) string, resumeFn func() (interface{}, error), responseFn func(code int, message string) interface{}) (interface{}, error) {
	serverAddress := getHeaderFn(faults.FaultServerAddressHeader)
	if serverAddress == "" || serverAddress != getShortenedAddress(address) {
		return resumeFn()
	}

	serverMethod := getHeaderFn(faults.FaultServerMethodHeader)
	if serverMethod != "" && serverMethod != method {
		return resumeFn()
	}

	delayMs := getHeaderFn(faults.FaultDelayMsHeader)
	if delayMs != "" {
		delayPercentage := getHeaderFn(faults.FaultDelayPercentageHeader)
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

	abortCode := getHeaderFn(faults.FaultAbortCodeHeader)
	if abortCode != "" {
		abortPercentage := getHeaderFn(faults.FaultAbortPercentageHeader)
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
		if code < 100 || code >= 600 {
			// log "provided abort code is outside the valid HTTP status code range of [100-599]"
			return resumeFn()
		}
		abortMessage := getHeaderFn(faults.FaultAbortMessageHeader)
		return responseFn(code, abortMessage), nil
	}

	return resumeFn()
}
