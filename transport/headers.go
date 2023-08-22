package transport

// Edge request context propagation related headers for gRPC and Thrift. For
// HTTP related headers refer to httpbp package.
// https://pages.github.snooguts.net/reddit/baseplate.spec/component-apis/thrift#edge-request-context-propagation
const (
	HeaderEdgeRequest = "Edge-Request"
	// The Trace ID, a 64-bit integer encoded in decimal.
	HeaderTracingTrace = "Trace"
	// The Span ID, a 64-bit integer encoded in decimal.
	HeaderTracingSpan = "Span"
	// The Parent Span ID, a 64-bit integer encoded in decimal.
	HeaderTracingParent = "Parent"
	// The Sampled flag, an ASCII "1" (HeaderTracingSampledTrue) if true,
	// otherwise false.
	// If not present, defaults to false.
	HeaderTracingSampled = "Sampled"
	// Trace flags, a 64-bit integer encoded in decimal.
	// If not present, defaults to null.
	HeaderTracingFlags = "Flags"
	// UserAgent related headers.
	HeaderUserAgent = "User-Agent"
	// HeaderTracingSampledTrue is the header value to indicate that this trace
	// should be sampled.
	HeaderTracingSampledTrue = "1"
	// Number of milliseconds, 64-bit integer encoded in decimal.
	HeaderDeadlineBudget = "Deadline-Budget"
	// Delegation tables (dtabs for short).
	// DTabs influence address resolution at request level, and are passed along
	// in the headers throughout the system. This allows us to do re-routing during
	// an in-flight request, at arbitrary depth of RPC call graph.
	HeaderDTabs = "DTabs"
)
