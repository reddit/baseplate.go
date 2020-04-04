package thriftbp

// Merge merges together multiple processors into the first one.
//
// It's useful when the server needs to support more than one separated thrift
// file.
//
// It's kind of like Apache Thrift's TMultiplexedProcessor, the key difference
// is that TMultiplexedProcessor requires the client to also use
// TMultiplexedProtocol, while here the client doesn't need any special
// handling.
func Merge(processors ...BaseplateProcessor) BaseplateProcessor {
	firstProcessor := processors[0]
	for i := 1; i < len(processors); i++ {
		processor := processors[i]
		for k, v := range processor.ProcessorMap() {
			firstProcessor.AddToProcessorMap(k, v)
		}
	}
	return firstProcessor
}
