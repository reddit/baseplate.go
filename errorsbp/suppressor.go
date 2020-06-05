package errorsbp

// Suppressor is a function that can be used to check if a given error should be
// suppressed/ignored.
//
// It is not reccomended that you define a Suppressor from scratch, rather you
// should compose one using Filters and NewSuppressor.
//
// Suppressors are not always necessary as what they do is fairly simple but can
// be useful when you want to ignore a subset of the potential errors something
// will have to handle in a way that is configurable.  For example, the metricsbp
// package in baseplate.go uses a Suppressor to determine if a Span should be
// considered a "failure" for the error it is passed.
type Suppressor func(err error) bool

// Filter is the building block of a Suppressor, multiple Filters are chained to
// create a new Suppressor.
//
// A Filter should return whether or not the error should be suppressed/ignored.
// If it cannot make that decision, it should call the 'next' Suppressor.
type Filter func(err error, next Suppressor) bool

func chain(f Filter, next Suppressor) Suppressor {
	return func(err error) bool {
		return f(err, next)
	}
}

func fallback(err error) bool {
	return false
}

// NewSuppressor return a Suppressor function using the given Filters.
//
// Filters will be executed in the order they are provided.
func NewSuppressor(filters ...Filter) Suppressor {
	suppressor := fallback
	for i := len(filters) - 1; i >= 0; i-- {
		suppressor = chain(filters[i], suppressor)
	}
	return suppressor
}
