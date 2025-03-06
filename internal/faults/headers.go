package faults

import (
	"fmt"
	"strconv"
	"strings"
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

	Only US-ASCII allowed, excluding semicolon <;> and comma <,>.

F = [Optional] Percentage chance to abort outgoing request, if matching.

	Only integers within [0-100] allowed.

Example:

	x-bp-fault: a=foo.bar;m=MyMethod;f=500;b=Fault injected!;F=50

	A request for MyMethod on service foo.bar will fail 50% of the time with a 500 response
	containing the body message "Fault injected!".
*/
const FaultHeader = "x-bp-fault"

type faultConfiguration struct {
	serverAddress   string
	serverMethod    string
	delayMs         int
	delayPercentage int
	abortCode       int
	abortMessage    string
	abortPercentage int
}

func (f *faultConfiguration) ServerAddress() string {
	return f.serverAddress
}
func (f *faultConfiguration) ServerMethod() string {
	return f.serverMethod
}
func (f *faultConfiguration) DelayMs() int {
	return f.delayMs
}
func (f *faultConfiguration) DelayPercentage() int {
	return f.delayPercentage
}
func (f *faultConfiguration) AbortCode() int {
	return f.abortCode
}
func (f *faultConfiguration) AbortMessage() string {
	return f.abortMessage
}
func (f *faultConfiguration) AbortPercentage() int {
	return f.abortPercentage
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

func parseMatchingFaultHeader(headerValue string, canonicalAddress, method string, abortCodeMin, abortCodeMax int) (*faultConfiguration, error) {
	if headerValue == "" {
		return nil, nil
	}

	config := &faultConfiguration{
		delayPercentage: 100,
		abortCode:       -1,
		abortPercentage: 100,
	}

	addressMatched := false

	parts := strings.Split(headerValue, ";")
	for _, part := range parts {
		keyValue := strings.Split(part, "=")
		if len(keyValue) != 2 {
			return nil, fmt.Errorf("invalid key-value pair: %q", part)
		}
		key, value := keyValue[0], keyValue[1]
		switch key {
		case "a":
			if value != canonicalAddress {
				return nil, nil
			}
			addressMatched = true
			config.serverAddress = value
		case "m":
			if method != "" && value != method {
				return nil, nil
			}
			config.serverMethod = value
		case "d":
			delayMs, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid delay value: %w", err)
			}
			config.delayMs = delayMs
		case "D":
			delayPercentage, err := parsePercentage(value)
			if err != nil {
				return nil, fmt.Errorf("error parsing delay percentage: %v", err)
			}
			config.delayPercentage = delayPercentage
		case "f":
			abortCode, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid abort code value: %w", err)
			}
			if abortCode < abortCodeMin || abortCode > abortCodeMax {
				return nil, fmt.Errorf("provided abort code \"%d\" is outside the valid range of [%d-%d]", abortCode, abortCodeMin, abortCodeMax)
			}
			config.abortCode = abortCode
		case "b":
			config.abortMessage = value
		case "F":
			abortPercentage, err := parsePercentage(value)
			if err != nil {
				return nil, fmt.Errorf("error parsing abort percentage: %v", err)
			}
			config.abortPercentage = abortPercentage
		default:
			return nil, fmt.Errorf("invalid key: %q", key)
		}
	}

	if addressMatched {
		return config, nil
	}
	return nil, nil
}

func parseMatchingFaultConfiguration(headerValues []string, canonicalAddress, method string, abortCodeMin, abortCodeMax int) (*faultConfiguration, error) {
	var errs error = nil
	for _, headerValue := range headerValues {
		// Additionally split combined values by comma, as per RFC 9110.
		splitHeaderValues := strings.Split(headerValue, ",")

		for _, splitHeaderValue := range splitHeaderValues {
			splitHeaderValue = strings.TrimSpace(splitHeaderValue)

			config, err := parseMatchingFaultHeader(splitHeaderValue, canonicalAddress, method, abortCodeMin, abortCodeMax)
			if err != nil {
				if errs == nil {
					errs = err
				} else {
					errs = fmt.Errorf("%v, %v", errs, err)
				}
			} else if config != nil {
				return config, errs
			}
		}
	}
	return nil, errs
}
