package faults

import (
	"errors"
	"fmt"
)

type errPercentageInvalidInt struct {
	percentage string
}

func (e *errPercentageInvalidInt) Error() string {
	return fmt.Sprintf("provided percentage %q is not a valid integer", e.percentage)
}

func (e *errPercentageInvalidInt) Is(err error) bool {
	other, ok := err.(*errPercentageInvalidInt)
	if !ok {
		return false
	}
	return e.percentage == other.percentage
}

type errPercentageOutOfRange struct {
	percentage int
}

func (e *errPercentageOutOfRange) Error() string {
	return fmt.Sprintf("provided percentage \"%d\" is outside the valid range of [0-100]", e.percentage)
}

func (e *errPercentageOutOfRange) Is(err error) bool {
	other, ok := err.(*errPercentageOutOfRange)
	if !ok {
		return false
	}
	return e.percentage == other.percentage
}

type errAbortCodeOutOfRange struct {
	abortCode    int
	abortCodeMin int
	abortCodeMax int
}

func (e *errAbortCodeOutOfRange) Error() string {
	return fmt.Sprintf("provided abort code \"%d\" is outside the valid range of [%d-%d]", e.abortCode, e.abortCodeMin, e.abortCodeMax)
}

func (e *errAbortCodeOutOfRange) Is(err error) bool {
	other, ok := err.(*errAbortCodeOutOfRange)
	if !ok {
		return false
	}
	return e.abortCode == other.abortCode && e.abortCodeMin == other.abortCodeMin && e.abortCodeMax == other.abortCodeMax
}

type errKVPairInvalid struct {
	part string
}

func (e *errKVPairInvalid) Error() string {
	return fmt.Sprintf("invalid key-value pair: %q", e.part)
}

func (e *errKVPairInvalid) Is(err error) bool {
	other, ok := err.(*errKVPairInvalid)
	if !ok {
		return false
	}
	return e.part == other.part
}

type errUnknownKey struct {
	key string
}

func (e *errUnknownKey) Error() string {
	return fmt.Sprintf("invalid key: %q", e.key)
}

func (e *errUnknownKey) Is(err error) bool {
	other, ok := err.(*errUnknownKey)
	if !ok {
		return false
	}
	return e.key == other.key
}

var (
	errDelayInvalid     = errors.New("invalid delay value")
	errAbortCodeInvalid = errors.New("invalid abort code value")
)
