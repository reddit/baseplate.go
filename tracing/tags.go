package tracing

import (
	"context"
	"fmt"
	"sync/atomic"
)

var alwaysIncludeAllowList = []string{
	TagKeyClient,
	TagKeyEndpoint,
}

// actual type: []string
var tagsAllowList atomic.Value

// SetMetricsTagsAllowList sets the allow-list used to carry tags from spans to
// metrics.
//
// "client" and "endpoint" are always included even if they are not in list.
//
// You should only set the tags you really need in metrics and limit the size of
// this allow-list. A big allow-list both makes span operations slower, and
// increase metrics cardinality.
func SetMetricsTagsAllowList(list []string) {
	tagsAllowList.Store(generateAllowList(list))
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
	v := tagsAllowList.Load()
	if v == nil {
		return alwaysIncludeAllowList
	}
	if l, ok := v.([]string); ok {
		return l
	}

	// Should not happen, but just in case
	globalTracer.logger.Log(
		context.Background(),
		fmt.Sprintf("Unexpected allow-list value: %#v", v),
	)
	return alwaysIncludeAllowList
}
