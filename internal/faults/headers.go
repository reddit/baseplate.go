package faults

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

/*
FaultHeader is the sole header used for fault injection configuration. It is formatted as a list
of <key=value> pairs delimited by <;>, with support for the following keys:

a = [Required] Server address of outgoing request.

	Used to determine whether the current request should have a fault injected.

m = [Optional] Method of outgoing request.

	Used to determine whether the current request should have a fault injected.

d = [Optional] Number of milliseconds to delay the outgoing request, if matching.

D = [Optional] Percentage chance to delay outgoing request, if matching.

	Only integers within [0-100] allowed.

f = [Optional] Abort current outgoing request and return this response code, if matching.

	Only integers within [0-599] allowed.

b = [Optional] Message to return with the aborted request response, if matching.

	Only US-ASCII allowed, excluding semicolon <;>, comma <,>, and equal sign <=>.

F = [Optional] Percentage chance to abort outgoing request, if matching.

	Only integers within [0-100] allowed.

Example:

	x-bp-fault: a=foo.bar;m=MyMethod;f=500;b=Fault injected!;F=50

	A request for MyMethod on service foo.bar will fail 50% of the time with a 500 response
	containing the body message "Fault injected!".
*/
const FaultHeader = "x-bp-fault"

var (
	errPercentageInvalidInt   = errors.New("provided percentage is not a valid integer")
	errPercentageOutOfRange   = errors.New("provided percentage is outside the valid range of [0-100]")
	errKVPairInvalid          = errors.New("invalid key-value pair")
	errDelayInvalid           = errors.New("invalid delay value")
	errDelayPercentageInvalid = errors.New("invalid delay percentage")
	errAbortCodeInvalid       = errors.New("invalid abort code value")
	errAbortCodeOutOfRange    = errors.New("provided abort code is outside the valid range")
	errAbortPercentageInvalid = errors.New("invalid abort percentage")
	errUnknownKey             = errors.New("unknown key")
)

type faultConfiguration struct {
	ServerAddress   string
	ServerMethod    string
	Delay           time.Duration
	DelayPercentage int
	AbortCode       int
	AbortMessage    string
	AbortPercentage int
}

func parsePercentage(percentage string) (int, error) {
	if percentage == "" {
		return 100, nil
	}
	intPercentage, err := strconv.Atoi(percentage)
	if err != nil {
		return 0, fmt.Errorf("%w: %q: %w", errPercentageInvalidInt, percentage, err)
	}
	if intPercentage < 0 || intPercentage > 100 {
		return 0, fmt.Errorf("%w: %q", errPercentageOutOfRange, intPercentage)
	}
	return intPercentage, nil
}

func parseMatchingFaultHeader(headerValue string, canonicalAddress, method string, abortCodeMin, abortCodeMax int) (*faultConfiguration, error) {
	if headerValue == "" {
		return nil, nil
	}

	config := &faultConfiguration{
		DelayPercentage: 100,
		AbortCode:       -1,
		AbortPercentage: 100,
	}

	addressMatched := false

	parts := strings.Split(headerValue, ";")
	for _, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("%w: %q", errKVPairInvalid, part)
		}
		switch key {
		case "a":
			if value != canonicalAddress {
				return nil, nil
			}
			addressMatched = true
			config.ServerAddress = value
		case "m":
			if value != method {
				return nil, nil
			}
			config.ServerMethod = value
		case "d":
			delayMs, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errDelayInvalid, err)
			}
			config.Delay = time.Duration(delayMs) * time.Millisecond
		case "D":
			delayPercentage, err := parsePercentage(value)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errDelayPercentageInvalid, err)
			}
			config.DelayPercentage = delayPercentage
		case "f":
			abortCode, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errAbortCodeInvalid, err)
			}
			if abortCode < abortCodeMin || abortCode > abortCodeMax {
				return nil, fmt.Errorf("%w: %d [%d-%d]", errAbortCodeOutOfRange, abortCode, abortCodeMin, abortCodeMax)
			}
			config.AbortCode = abortCode
		case "b":
			config.AbortMessage = value
		case "F":
			abortPercentage, err := parsePercentage(value)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errAbortPercentageInvalid, err)
			}
			config.AbortPercentage = abortPercentage
		default:
			return nil, fmt.Errorf("%w: %q", errUnknownKey, key)
		}
	}

	if addressMatched {
		return config, nil
	}
	return nil, nil
}

func parseMatchingFaultConfiguration(headerValues []string, canonicalAddress, method string, abortCodeMin, abortCodeMax int) (*faultConfiguration, error) {
	var errs []error
	for _, headerValue := range headerValues {
		// Additionally split combined values by comma, as per RFC 9110.
		splitHeaderValues := strings.Split(headerValue, ",")

		for _, splitHeaderValue := range splitHeaderValues {
			splitHeaderValue = strings.TrimSpace(splitHeaderValue)

			config, err := parseMatchingFaultHeader(splitHeaderValue, canonicalAddress, method, abortCodeMin, abortCodeMax)
			if err != nil {
				errs = append(errs, err)
			} else if config != nil {
				return config, errors.Join(errs...)
			}
		}
	}
	return nil, errors.Join(errs...)
}
