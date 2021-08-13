package errorsbp

// PrefixError appends prefix to err.
//
// If err is nil, nil will be returned.
// If prefix is empty, err will be returned as-is.
// Otherwise, the returned error will be of type *PrefixedError,
// and the error message would be:
//
//     "prefix: err.Error()"
//
// NOTE: A reason to use PrefixError over fmt.Errorf(prefix + ": %w", err)
// is that prefix could contain format verbs, e.g. prefix = "foo%sbar".
func PrefixError(prefix string, err error) error {
	if err == nil {
		return nil
	}

	if prefix == "" {
		return err
	}

	return &PrefixedError{
		prefix: prefix,
		msg:    prefix + ": " + err.Error(),
		err:    err,
	}
}

// PrefixedError defines the type of error returned by PrefixError.
type PrefixedError struct {
	prefix string
	msg    string
	err    error
}

func (e *PrefixedError) Error() string {
	return e.msg
}

func (e *PrefixedError) Unwrap() error {
	return e.err
}

// Prefix returns the prefix of this error.
func (e *PrefixedError) Prefix() string {
	return e.prefix
}
