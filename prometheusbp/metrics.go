package prometheusbp

// BoolString returns the string version of a boolean value that should be used
// in a prometheus metric label.
func BoolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
