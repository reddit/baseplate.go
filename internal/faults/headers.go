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
of segments delimited by <;>, with support for the following keys:

a= / a!= [Required] Server address of outgoing request.

	Used to determine whether the current request should have a fault injected. Multiple address
	matchers may be provided; all must match.

h= / h!= [Optional] Host/authority header of outgoing request.

	Used to determine whether the current request should have a fault injected. This is most useful when the address is a proxy and the host/authority header represents the true server target.

m= / m!= [Optional] Method of outgoing request.

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

Inequality matchers
-------------------

The matcher keys "a", "h", and "m" accept the operator "!=" in place of "=" to require that
the request value differ from the configured value:

	x-bp-fault: a=foo.bar;m!=Healthcheck;f=500

	Every method on service foo.bar except Healthcheck will fail with 500.
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
	errExtraEqualsInValue     = errors.New("invalid key-value pair with extra equals")
	errInequalityActionKey    = errors.New("inequality matcher not allowed on action key")
)

type faultConfiguration struct {
	ServerAddress   string
	ServerHost      string
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

func splitSegment(s string) (key, value string, isInequality bool, err error) {
	key, value, ok := strings.Cut(s, "=")
	if !ok {
		return "", "", false, fmt.Errorf("segment %q: %w", s, errKVPairInvalid)
	}
	if strings.HasSuffix(key, "!") {
		isInequality = true
		key = strings.TrimSuffix(key, "!")
	}
	if strings.Contains(key, "!") {
		return "", "", false, fmt.Errorf("segment %q: %w", s, errKVPairInvalid)
	}
	return key, value, isInequality, nil
}

func matcherMatches(actual, configured string, isInequality bool) bool {
	if isInequality {
		return actual != configured
	}
	return actual == configured
}

func parseMatchingFaultHeader(headerValue string, canonicalAddress, host, method string, abortCodeMin, abortCodeMax int) (*faultConfiguration, error) {
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
		part = strings.TrimSpace(part)
		key, value, isInequality, err := splitSegment(part)
		if err != nil {
			return nil, err
		}
		if key != "b" && strings.Contains(value, "=") {
			return nil, fmt.Errorf("segment %q: %w", part, errExtraEqualsInValue)
		}
		switch key {
		case "a":
			if !matcherMatches(canonicalAddress, value, isInequality) {
				return nil, nil
			}
			addressMatched = true
			if !isInequality {
				config.ServerAddress = value
			}
		case "h":
			if !matcherMatches(host, value, isInequality) {
				return nil, nil
			}
			if !isInequality {
				config.ServerHost = value
			}
		case "m":
			if !matcherMatches(method, value, isInequality) {
				return nil, nil
			}
			if !isInequality {
				config.ServerMethod = value
			}
		case "d", "D", "f", "b", "F":
			if isInequality {
				return nil, fmt.Errorf("action key %q: %w", key, errInequalityActionKey)
			}
			switch key {
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
			}
		default:
			return nil, fmt.Errorf("%w: %q", errUnknownKey, key)
		}
	}

	if addressMatched {
		return config, nil
	}
	return nil, nil
}

func parseMatchingFaultConfiguration(headerValues []string, canonicalAddress, host, method string, abortCodeMin, abortCodeMax int) (*faultConfiguration, error) {
	var errs []error
	for _, headerValue := range headerValues {
		// Additionally split combined values by comma, as per RFC 9110.
		splitHeaderValues := strings.Split(headerValue, ",")

		for _, splitHeaderValue := range splitHeaderValues {
			splitHeaderValue = strings.TrimSpace(splitHeaderValue)

			config, err := parseMatchingFaultHeader(splitHeaderValue, canonicalAddress, host, method, abortCodeMin, abortCodeMax)
			if err != nil {
				errs = append(errs, err)
			} else if config != nil {
				return config, errors.Join(errs...)
			}
		}
	}
	return nil, errors.Join(errs...)
}
