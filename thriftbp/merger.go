package thriftbp

import (
	"github.com/apache/thrift/lib/go/thrift"
)

// Merge merges together the specified processors.
//
// It's useful if the server need to support two separated thrift files.
//
// It's kind of like Apache Thrift's TMultiplexedProcessor, the key difference
// is that TMultiplexedProcessor requires the client to also use
// TMultiplexedProtocol, while here the client didn't need any special handling.
func Merge(processors ...BaseplateProcessor) thrift.TProcessor {
	firstProcessor := processors[0]
	for i := 1; i < len(processors); i++ {
		processor := processors[i]
		for k, v := range processor.ProcessorMap() {
			firstProcessor.AddToProcessorMap(k, v)
		}
	}
	return firstProcessor
}
