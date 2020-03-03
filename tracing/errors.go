package tracing

import (
	"fmt"
)

// InvalidSpanTypeError is the error type returned when trying to use a Span in
// a way that is incompatible with its type.
//
// For example, trying to set a child span as a ServerSpan.
type InvalidSpanTypeError struct {
	ExpectedSpanType SpanType
	ActualSpanType   SpanType
}

var _ error = (*InvalidSpanTypeError)(nil)

func (e *InvalidSpanTypeError) Error() string {
	return fmt.Sprintf(
		"span.SpanType: expected (%v) != actual (%v)",
		e.ExpectedSpanType,
		e.ActualSpanType,
	)
}
