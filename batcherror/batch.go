package batcherror

import (
	"errors"
	"fmt"
	"strings"
)

// Make sure both BatchError and *BatchError satisfies error interface.
var (
	_ error = BatchError{}
	_ error = (*BatchError)(nil)
)

// BatchError is an error that can contain multiple errors.
//
// The zero value of BatchError is valid (with no errors) and ready to use.
//
// This type is not thread-safe.
// The same batch should not be operated on different goroutines concurrently.
type BatchError struct {
	errors []error
}

func (be BatchError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(
		&sb,
		"batcherror: total %d error(s) in this batch",
		len(be.errors),
	)
	for i, err := range be.errors {
		if i == 0 {
			sb.WriteString(": ")
		} else {
			sb.WriteString("; ")
		}
		fmt.Fprintf(&sb, "%+v", err)
	}
	return sb.String()
}

// As implements helper interface for errors.As.
//
// It supports pointers to both BatchError and *BatchError.
func (be BatchError) As(v interface{}) bool {
	if target, ok := v.(*BatchError); ok {
		target.errors = be.errors
		return true
	}
	if target, ok := v.(**BatchError); ok {
		*target = &be
		return true
	}
	return false
}

// Unwrap implements the hidden errors interface.
//
// When the batch contains exactly one error, that error is returned.
// It returns nil otherwise.
func (be BatchError) Unwrap() error {
	if len(be.errors) == 1 {
		return be.errors[0]
	}
	return nil
}

func (be *BatchError) addBatch(batch BatchError) {
	be.errors = append(be.errors, batch.errors...)
}

// Add adds an error into the batch.
//
// If the error is also an BatchError,
// its underlying error(s) will be added instead of the BatchError itself.
//
// Nil error will be skipped.
func (be *BatchError) Add(err error) {
	if err == nil {
		return
	}

	var batch BatchError
	if errors.As(err, &batch) {
		be.addBatch(batch)
	} else {
		be.errors = append(be.errors, err)
	}
}

// Compile compiles the batch.
//
// If the batch contains zero errors, Compile returns nil.
//
// If the batch contains exactly one error,
// that underlying error will be returned.
//
// Otherwise, the batch itself will be returned.
func (be BatchError) Compile() error {
	switch len(be.errors) {
	case 0:
		return nil
	case 1:
		return be.errors[0]
	default:
		return be
	}
}

// Clear clears the batch.
func (be *BatchError) Clear() {
	be.errors = nil
}

// GetErrors returns a copy of the underlying error(s).
func (be BatchError) GetErrors() []error {
	errors := make([]error, len(be.errors))
	copy(errors, be.errors)
	return errors
}
