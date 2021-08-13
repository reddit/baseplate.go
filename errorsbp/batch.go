package errorsbp

import (
	"errors"
	"fmt"
	"strings"
)

// Make sure both Batch and *Batch satisfies error interface.
var (
	_ error = Batch{}
	_ error = (*Batch)(nil)
)

// Batch is an error that can contain multiple errors.
//
// The zero value of Batch is valid (with no errors) and ready to use.
//
// This type is not thread-safe.
// The same batch should not be operated on different goroutines concurrently.
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
// It uses PrefixError under the hood.
//
// It's useful in Closer implementations that need to call multiple Closers.
func (be *Batch) AddPrefix(prefix string, errs ...error) {
	if prefix == "" {
		be.Add(errs...)
		return
	}

	appendSingle := func(err error) {
		be.errors = append(be.errors, PrefixError(prefix, err))
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
