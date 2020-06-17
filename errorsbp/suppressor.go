package errorsbp

// Suppressor defines a type of function can be used to suppress certain errors.
//
// The implementation shall return true on the errors they want to suppress,
// and false on all other errors.
type Suppressor func(err error) bool

// Suppress is the nil-safe way of calling a Suppressor.
func (s Suppressor) Suppress(err error) bool {
	if s != nil {
		return s(err)
	}
	return SuppressNone(err)
}

// Wrap wraps the error based on the decision of Suppressor.
//
// If the error shall be suppressed, Wrap returns nil.
// Otherwise Wrap returns the error as-is.
//
// Like Suppress, Wrap is also nil-safe.
func (s Suppressor) Wrap(err error) error {
	if s.Suppress(err) {
		return nil
	}
	return err
}

// SuppressNone is a Suppressor implementation that always return false,
// thus suppress none of the errors.
//
// It's the default implementation nil Suppressor falls back into.
func SuppressNone(err error) bool {
	return false
}

var _ Suppressor = SuppressNone

// OrSuppressors combines the given suppressors.
//
// If any of the suppressors return true on an error,
// the combined Suppressor would returns true on that error.
func OrSuppressors(suppressors ...Suppressor) Suppressor {
	return func(err error) bool {
		for _, s := range suppressors {
			if s.Suppress(err) {
				return true
			}
		}
		return false
	}
}
