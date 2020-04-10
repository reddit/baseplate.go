package metricsbp

// Labels allows you to specify labels as a convenient map and
// provides helpers to convert them into other formats.
type Labels map[string]string

// AsStatsdLabels returns the labels in the format expected by the
// statsd metrics client, that is a slice of strings.
//
// This method is nil-safe and will just return nil if the receiver is
// nil.
func (l Labels) AsStatsdLabels() []string {
	if l == nil {
		return nil
	}
	labels := make([]string, 0, len(l)*2)
	for k, v := range l {
		labels = append(labels, k, v)
	}
	return labels
}
