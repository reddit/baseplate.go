package tracing

import (
	"sync/atomic"
)

var alwaysIncludeAllowList = []string{
	TagKeyClient,
	TagKeyEndpoint,
}

// NOTE: In general storing a pointer of []string to an atomic value does not
// really guarantee atomicy because a slice is a container and its content can
// be modified (e.g. replace individual strings inside). A copy of the slice is
// required to guarantee atomicy.
//
// But here, when storing it we always create a new string (via
// generateAllowList), and the read function (getAllowList) is unexported and we
// never mutate the returned result, so skip copying is OK.
//
// But if any of those assumptions are broken in the future, we can no longer
// store the atomic.Pointer[[]string] naively, and would need to either make a
// copy or add other protections.
var tagsAllowList atomic.Pointer[[]string]

// SetMetricsTagsAllowList sets the allow-list used to carry tags from spans to
// metrics.
//
// "client" and "endpoint" are always included even if they are not in list.
//
// You should only set the tags you really need in metrics and limit the size of
// this allow-list. A big allow-list both makes span operations slower, and
// increase metrics cardinality.
func SetMetricsTagsAllowList(list []string) {
	v := generateAllowList(list)
	tagsAllowList.Store(&v)
}

func generateAllowList(list []string) []string {
	m := make(map[string]struct{}, len(list)+len(alwaysIncludeAllowList))
	var value struct{}
	for _, tag := range alwaysIncludeAllowList {
		m[tag] = value
	}
	for _, tag := range list {
		m[tag] = value
	}
	l := make([]string, 0, len(m))
	for tag := range m {
		l = append(l, tag)
	}
	return l
}

func getAllowList() []string {
	if v := tagsAllowList.Load(); v != nil {
		return *v
	}
	return alwaysIncludeAllowList
}
