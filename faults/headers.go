package faults

const (
	// General
	FaultServerAddressHeader   = "X-Bp-Fault-Server-Address"
	FaultDelayMsHeader         = "X-Bp-Fault-Delay-Ms"
	FaultAbortCodeHeader       = "X-Bp-Fault-Abort-Code"
	FaultServerMethodHeader    = "X-Bp-Fault-Server-Method"
	FaultAbortMessageHeader    = "X-Bp-Fault-Abort-Message"
	FaultDelayPercentageHeader = "X-Bp-Fault-Delay-Percentage"
	FaultAbortPercentageHeader = "X-Bp-Fault-Abort-Percentage"

	// Thrift-specific
	FaultThriftErrorTypeHeader = "X-Bp-Fault-Thrift-Error-Type"
)
