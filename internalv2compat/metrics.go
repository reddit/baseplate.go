package internalv2compat

import (
	"github.com/prometheus/client_golang/prometheus"
)

// GlobalRegistry should be used to register all metrics from baseplate Go v0
// to ensure they do not conflict with metrics from baseplate Go v2 libraries
// which will have the same name.
var GlobalRegistry = prometheus.WrapRegistererWith(prometheus.Labels{
	"baseplate_go": "v0",
}, prometheus.DefaultRegisterer)
