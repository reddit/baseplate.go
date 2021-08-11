package redisx

import (
	"reflect"

	"github.com/reddit/baseplate.go/retrybp"
)

// InvalidInputError is returned when the response input is invalid.
// This check happens after the command has already been processed by Redis
// while processing the input.
type InvalidInputError struct {
	Message string
}

// Error implements the error interface.
func (e *InvalidInputError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = "received an invalid response input"
	}
	return "redisx: " + msg
}

// Retryable implements retrybp.RetryableError. InvalidInputError is never retryable.
func (e *InvalidInputError) Retryable() int {
	return -1
}

// ResponseInputTypeError is returned when response input is valid but the
// command does not support the response input type passed in to it.
// This check happens after the command has already been processed by Redis
// while processing the input.
type ResponseInputTypeError struct {
	Cmd               string
	ResponseInputType reflect.Type
}

// Error implements the error interface.
func (e *ResponseInputTypeError) Error() string {
	return "redisx: command " + e.Cmd + " does not support the response input type " + e.ResponseInputType.String()
}

// Retryable implements retrybp.RetryableError. ResponseInputTypeError is never retryable.
func (e *ResponseInputTypeError) Retryable() int {
	return -1
}

// UnexpectedResponseError is returned when the response we received from
// redispipe does not match what we expect or is invalid.
// This check happens after the command has already been processed by Redis
// while processing the input.
//
// You should not encounter this error and if you do, it likely indicates a bug
// in redisx or, less likely, redispipe.
// Please report this in  https://github.com/reddit/baseplate.go/issues if you
// encounter it with any details.
type UnexpectedResponseError struct {
	Message string
}

// Error implements the error interface.
func (e *UnexpectedResponseError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = "received an unexpected response from redispipe"
	}
	return "redisx: " + msg
}

// Retryable implements retrybp.RetryableError. UnexpectedResponseError is never retryable.
func (e *UnexpectedResponseError) Retryable() int {
	return -1
}

var (
	_ retrybp.RetryableError = (*InvalidInputError)(nil)
	_ retrybp.RetryableError = (*ResponseInputTypeError)(nil)
	_ retrybp.RetryableError = (*UnexpectedResponseError)(nil)
)
