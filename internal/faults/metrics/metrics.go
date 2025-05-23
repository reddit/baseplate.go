package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
)

const (
	promNamespace      = "faultbp"
	clientNameLabel    = "fault_client_name"
	serviceLabel       = "fault_service"
	methodLabel        = "fault_method"
	protocolLabel      = "fault_protocol"
	statusLabel        = "fault_status"
	delayInjectedLabel = "fault_injected_delay"
	abortInjectedLabel = "fault_injected_abort"
)

type FaultStatus int

const (
	Success FaultStatus = iota
	HeaderLookupError
	ConfigParsingError
	NoMatchingConfig
	DelayError
)

func (fs FaultStatus) String() string {
	switch fs {
	case Success:
		return "success"
	case HeaderLookupError:
		return "header_lookup_error"
	case ConfigParsingError:
		return "config_parsing_error"
	case NoMatchingConfig:
		return "no_matching_config"
	case DelayError:
		return "delay_error"
	default:
		return "unknown"
	}
}

var (
	TotalRequests = promauto.With(prometheusbpint.GlobalRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "faultbp_fault_requests_total",
		Help: "Total count of requests seen by the fault injection middleware.",
	}, []string{
		clientNameLabel,
		serviceLabel,
		methodLabel,
		protocolLabel,
		statusLabel,
		delayInjectedLabel,
		abortInjectedLabel,
	})
)
