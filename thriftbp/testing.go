package thriftbp

import (
	"context"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
)

type mockContextKey int

const (
	processorName mockContextKey = iota
)

// SetMockBaseplateProcessorName sets the "name" of the TProcessorFunction to call on
// a MockBaseplateProcessor when calling Process.
//
// In a normal TProcessor, the request name is read from the request itself
// which happens in TProcessor.Process, so it is not passed into the call to
// Process itself, to get around this, MockBaseplateProcessor calls GetMockBaseplateProcessorName
// to get the name to use from the context object.
func SetMockBaseplateProcessorName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, processorName, name)
}

// GetMockBaseplateProcessorName gets the "name" of the TProcessorFunction to call on a
// MockBaseplateProcessor when calling Process.
func GetMockBaseplateProcessorName(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(processorName).(string)
	return val, ok
}

// MockBaseplateProcessor can be used to create a mock object that fufills the
// BaseplateProcessor interface in testing.
type MockBaseplateProcessor struct {
	processorMap map[string]thrift.TProcessorFunction
}

// NewMockBaseplateProcessor returns a pointer to a new MockBaseplateProcessor
// object with the internal processor map initialized to the one passed in.
//
// If the passed in map is nil, an empty map will be initialized and passed in
// to the new object so it is safe to add to.
func NewMockBaseplateProcessor(processorMap map[string]thrift.TProcessorFunction) *MockBaseplateProcessor {
	if processorMap == nil {
		processorMap = make(map[string]thrift.TProcessorFunction)
	}
	return &MockBaseplateProcessor{processorMap: processorMap}
}

// Process calls the TProcessorFunction assigned to the "name" set on the
// context object by SetMockBaseplateProcessorName.
//
// If no name is set on the context or there is no TProcessorFunction mapped to
// that name, the call will panic.
func (p *MockBaseplateProcessor) Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException) {
	name, ok := GetMockBaseplateProcessorName(ctx)
	if !ok {
		panic("MockBaseplateProcessorName not set on context")
	}
	processor, ok := p.ProcessorMap()[name]
	if !ok {
		panic(fmt.Sprintf("No processor set for name %q", name))
	}
	return processor.Process(ctx, 0, in, out)
}

// ProcessorMap returns the internal processor map.
func (p *MockBaseplateProcessor) ProcessorMap() map[string]thrift.TProcessorFunction {
	return p.processorMap
}

// AddToProcessorMap adds the given TProcessorFunction to the internal processor
// map with the given name as the key.
func (p *MockBaseplateProcessor) AddToProcessorMap(name string, processorFunc thrift.TProcessorFunction) {
	p.processorMap[name] = processorFunc
}

var (
	_ BaseplateProcessor = (*MockBaseplateProcessor)(nil)
)
