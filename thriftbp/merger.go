package thriftbp

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
)

// MergeableTProcessor interface contains the functions needed to merge
// processors together.
//
// It is used by the Merge function below.
type MergeableTProcessor interface {
	Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException)
	AddToProcessorMap(key string, processor thrift.TProcessorFunction)
	ProcessorMap() map[string]thrift.TProcessorFunction
}

// Merge merges together the specified processors.
//
// It's useful if the server need to support two separated thrift files.
//
// It's kind of like Apache Thrift's TMultiplexedProcessor, the key difference
// is that TMultiplexedProcessor requires the client to also use
// TMultiplexedProtocol, while here the client didn't need any special handling.
func Merge(processors ...MergeableTProcessor) thrift.TProcessor {
	firstProcessor := processors[0]
	for i := 1; i < len(processors); i++ {
		processor := processors[i]
		for k, v := range processor.ProcessorMap() {
			firstProcessor.AddToProcessorMap(k, v)
		}
	}
	return firstProcessor
}
