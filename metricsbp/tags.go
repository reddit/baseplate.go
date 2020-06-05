package metricsbp

// Tags allows you to specify tags as a convenient map and
// provides helpers to convert them into other formats.
type Tags map[string]string

// AsStatsdTags returns the tags in the format expected by the
// statsd metrics client, that is a slice of strings.
//
// This method is nil-safe and will just return nil if the receiver is
// nil.
func (t Tags) AsStatsdTags() []string {
	if t == nil {
		return nil
	}
	tags := make([]string, 0, len(t)*2)
	for k, v := range t {
		tags = append(tags, k, v)
	}
	return tags
}
