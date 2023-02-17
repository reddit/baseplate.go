package errorsbp

import (
	"errors"
	"fmt"
	"strings"
)

type batchUnwrapper interface {
	error

	Unwrap() []error
}

// Make sure both Batch and *Batch satisfies error and batchUnwrapper interfaces.
var (
	_ batchUnwrapper = Batch{}
	_ batchUnwrapper = (*Batch)(nil)
)

// Batch is an error that can contain multiple errors.
//
// The zero value of Batch is valid (with no errors) and ready to use.
//
// This type is not thread-safe.
// The same batch should not be operated on different goroutines concurrently.
//
// To be deprecated when we drop support for go 1.19.
type Batch struct {
	errors []error
}

func (be Batch) Error() string {
	var sb strings.Builder
	fmt.Fprintf(
		&sb,
		"errorsbp.Batch: total %d error(s) in this batch",
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

// Len returns the size of the batch.
//
// Note that this is the naive size without traversal recursively.
// See BatchSize for the accumulated size with recursive traversal instead.
func (be Batch) Len() int {
	return len(be.errors)
}

// As implements helper interface for errors.As.
//
// If v is pointer to either Batch or *Batch,
// *v will be set into this error.
// Otherwise, As will try errors.As against all errors in this batch,
// returning the first match.
//
// See Is for the discussion of possiblity of infinite loop.
func (be Batch) As(v interface{}) bool {
	if target, ok := v.(*Batch); ok {
		*target = be
		return true
	}
	if target, ok := v.(**Batch); ok {
		*target = &be
		return true
	}
	for _, err := range be.errors {
		if errors.As(err, v) {
			return true
		}
	}
	return false
}

// Is implements helper interface for errors.Is.
//
// It calls errors.Is against all errors in this batch,
// until a match is found.
//
// If an error in the batch is the Batch itself,
// calling its Is (and As) could cause an infinite loop.
// But there's a special handling in Add function,
// that if you try to add a Batch into the batch,
// we add the underlying errors instead the Batch itself.
// As a result it should be impossible to cause infinite loops in Is and As.
func (be Batch) Is(target error) bool {
	for _, err := range be.errors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func (be *Batch) addBatch(batch Batch) {
	be.errors = append(be.errors, batch.errors...)
}

// Add adds errors into the batch.
//
// If an error is also a Batch,
// its underlying error(s) will be added instead of the Batch itself.
//
// Nil errors will be skipped.
func (be *Batch) Add(errs ...error) {
	for _, err := range errs {
		if err == nil {
			continue
		}

		var batch Batch
		if errors.As(err, &batch) {
			be.addBatch(batch)
		} else {
			be.errors = append(be.errors, err)
		}
	}
}

// AddPrefix adds errors into the batch with given prefix.
//
// If an error is also a Batch,
// its underlying error(s) will be added instead of the Batch itself,
// all with the same given prefix.
//
// Nil errors will be skipped.
//
// The actual error(s) added into the batch will produce the error message of:
//
//	"prefix: err.Error()"
//
// It's useful in Closer implementations that need to call multiple Closers.
func (be *Batch) AddPrefix(prefix string, errs ...error) {
	if prefix == "" {
		be.Add(errs...)
		return
	}

	appendSingle := func(err error) {
		be.errors = append(be.errors, Prefix(prefix, err))
	}

	for _, err := range errs {
		if err == nil {
			continue
		}

		var batch Batch
		if errors.As(err, &batch) {
			for _, err := range batch.errors {
				appendSingle(err)
			}
		} else {
			appendSingle(err)
		}
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
func (be Batch) Compile() error {
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
func (be *Batch) Clear() {
	be.errors = nil
}

// GetErrors returns a copy of the underlying error(s).
func (be Batch) GetErrors() []error {
	errors := make([]error, len(be.errors))
	copy(errors, be.errors)
	return errors
}

// Unwrap implements the optional interface defined in go 1.20.
//
// It's an alias to GetErrors.
func (be Batch) Unwrap() []error {
	return be.GetErrors()
}

// BatchSize returns the size of the batch for error err.
//
// If err implements `Unwrap() []error` (optional interface defined in go 1.20),
// or it unwraps to an error that implements `Unwrap() []error`
// (which includes errorsbp.Batch and *errorsbp.Batch),
// BatchSize returns the total size of Unwrap'd errors recursively.
// Otherwise, it returns 1 if err is non-nil, and 0 if err is nil.
//
// Note that for a Batch,
// it's possible that BatchSize returns a different (higher) size than its Len.
// That would only happen if the Batch contains batch errors generated by other
// implementation(s) (for example, errors.Join or fmt.Errorf).
//
// It's useful in tests,
// for example to verify that a function indeed returns the exact number of
// errors as expected.
//
// It's possible to construct an error to cause BatchSize to recurse infinitely
// and thus cause stack overflow, so in general BatchSize should only be used in
// test code and not in production code.
func BatchSize(err error) int {
	if err == nil {
		return 0
	}
	var unwrapper batchUnwrapper
	if errors.As(err, &unwrapper) {
		// Since neither errors.Join nor fmt.Errorf tries to flatten the errors when
		// combining them, do this recursively instead of simply return
		// len(unwrapper.Unwrap()).
		var total int
		for _, e := range unwrapper.Unwrap() {
			total += BatchSize(e)
		}
		return total
	}
	// single, non-batch error.
	return 1
}

// Prefix is a helper function to add prefix to a potential error.
//
// If err is nil, it returns nil.
// If prefix is empty string, it returns err as-is.
// Otherwise it returns an error that can unwrap to err with message of
// "prefix: err.Error()".
//
// It's useful to be used with errors.Join.
func Prefix(prefix string, err error) error {
	if err == nil {
		return nil
	}
	if prefix == "" {
		return err
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
