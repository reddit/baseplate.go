package thrifttest

import (
	"context"
	"errors"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/clientpool"
	"github.com/reddit/baseplate.go/thriftbp"
)

type mockContextKey int

const (
	processorName mockContextKey = iota
)

// SetMockTProcessorName sets the "name" of the TProcessorFunction to
// call on a MockTProcessor when calling Process.
//
// In a normal TProcessor, the request name is read from the request itself
// which happens in TProcessor.Process, so it is not passed into the call to
// Process itself, to get around this, MockTProcessor calls
// GetMockTProcessorName to get the name to use from the context object.
func SetMockTProcessorName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, processorName, name)
}

// GetMockTProcessorName gets the "name" of the TProcessorFunction to
// call on a MockTProcessor when calling Process.
func GetMockTProcessorName(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(processorName).(string)
	return val, ok
}

// MockTProcessor can be used to create a mock object that fufills the
// BaseplateProcessor interface in testing.
type MockTProcessor struct {
	tb           testing.TB
	processorMap map[string]thrift.TProcessorFunction
}

// NewMockTProcessor returns a pointer to a new MockTProcessor
// object with the internal processor map initialized to the one passed in.
//
// If the passed in map is nil, an empty map will be initialized and passed in
// to the new object so it is safe to add to.
func NewMockTProcessor(tb testing.TB, processorMap map[string]thrift.TProcessorFunction) *MockTProcessor {
	if processorMap == nil {
		processorMap = make(map[string]thrift.TProcessorFunction)
	}
	return &MockTProcessor{
		tb:           tb,
		processorMap: processorMap,
	}
}

// Process calls the TProcessorFunction assigned to the "name" set on the
// context object by SetMockTProcessorName.
//
// If no name is set on the context or there is no TProcessorFunction mapped to
// that name, the call will fail the test.
func (p *MockTProcessor) Process(ctx context.Context, in, out thrift.TProtocol) (bool, thrift.TException) {
	name, ok := GetMockTProcessorName(ctx)
	if !ok {
		p.tb.Fatal("MockTProcessorName not set on context")
	}
	processor, ok := p.ProcessorMap()[name]
	if !ok {
		p.tb.Fatalf("No processor set for name %q", name)
	}
	return processor.Process(ctx, 0, in, out)
}

// ProcessorMap returns the internal processor map.
func (p *MockTProcessor) ProcessorMap() map[string]thrift.TProcessorFunction {
	return p.processorMap
}

// AddToProcessorMap adds the given TProcessorFunction to the internal processor
// map with the given name as the key.
func (p *MockTProcessor) AddToProcessorMap(name string, processorFunc thrift.TProcessorFunction) {
	p.processorMap[name] = processorFunc
}

var (
	_ thrift.TProcessor = (*MockTProcessor)(nil)
)

// MockCall is a mock function that can be registered to a method in a
// MockClient.
type MockCall func(ctx context.Context, args, result thrift.TStruct) (thrift.ResponseMeta, error)

// MockClient implements thrift.TClient and Client,
// and can be used to mock out thrift calls in testing by using it as the base
// client rather than a real client.
//
// If a MockCall is registered to a method,
// then Call will return the result of that MockCall when that method is Call-ed.
// If no MockCall is registered to a method,
// Call will simply return nil when FailUnregisteredMethods is false,
// or an error when FailUnregisteredMethods is true.
//
// MockClient is provided to help with unit testing and should not be used in
// production code.
type MockClient struct {
	FailUnregisteredMethods bool

	methods map[string]MockCall
}

func nopMockCall(ctx context.Context, args, result thrift.TStruct) (meta thrift.ResponseMeta, err error) {
	return
}

// AddNopMockCalls registers the nop MockCall to the given methods.
//
// A nop MockCall is a MockCall implementation that does nothing and returns nil
// error.
func (c *MockClient) AddNopMockCalls(methods ...string) {
	for _, method := range methods {
		c.AddMockCall(method, nopMockCall)
	}
}

// AddMockCall registers the given MockCall to the given method.
//
// If a mock is already registered to that method,
// it will be replaced with the new mock.
//
// AddMockCall is not thread-safe.
func (c *MockClient) AddMockCall(method string, mock MockCall) {
	if c.methods == nil {
		c.methods = make(map[string]MockCall)
	}
	c.methods[method] = mock
}

// Call implements the thrift.TClient interface.
//
// It will return the result of the MockCall registered to method if one exists.
// If the method is not registered,
// it returns an error when FailUnregisteredMethods is true,
// nil otherwise.
func (c *MockClient) Call(ctx context.Context, method string, args, result thrift.TStruct) (thrift.ResponseMeta, error) {
	if m, ok := c.methods[method]; ok {
		return m(ctx, args, result)
	}
	if c.FailUnregisteredMethods {
		return thrift.ResponseMeta{}, errors.New("thrifttest.MockClient: unregistered method: " + method)
	}
	return thrift.ResponseMeta{}, nil
}

// Close implements Client and is a nop that always returns nil.
func (MockClient) Close() error {
	return nil
}

// IsOpen implements Client and is a nop that always returns true.
func (MockClient) IsOpen() bool {
	return true
}

// RecordedCall records the inputs passed to RecordedClient.RecordedCall.
type RecordedCall struct {
	Ctx    context.Context
	Method string
	args   thrift.TStruct
	result thrift.TStruct
}

// RecordedClient implements the thrift.TClient interface and records the inputs
// to each Call.
//
// RecordedClient is provided to help with unit testing and should not be used
// in production code.
//
// RecordedClient is not thread-safe.
type RecordedClient struct {
	client thrift.TClient

	calls []RecordedCall
}

// NewRecordedClient returns a pointer to a new RecordedClient that wraps the
// provided client.
func NewRecordedClient(c thrift.TClient) *RecordedClient {
	return &RecordedClient{client: c}
}

// Calls returns a copy of all of recorded Calls.
//
// Calls is not thread-safe and may panic if used across threads if the number
// of calls changes between the copy buffer being intialized and copied.
func (c RecordedClient) Calls() []RecordedCall {
	calls := make([]RecordedCall, len(c.calls))
	copy(calls, c.calls)
	return calls
}

// Call fufills the thrift.TClient interface. It will record the inputs to Call
// and either return the result of the inner client.Call or nil if the inner
// client is nil.
func (c *RecordedClient) Call(ctx context.Context, method string, args, result thrift.TStruct) (thrift.ResponseMeta, error) {
	c.calls = append(c.calls, RecordedCall{
		Ctx:    ctx,
		Method: method,
		args:   args,
		result: result,
	})
	if c.client != nil {
		return c.client.Call(ctx, method, args, result)
	}
	return thrift.ResponseMeta{}, nil
}

// MockClientPool is a ClientPool implementation can be used in test code.
type MockClientPool struct {
	Exhausted    bool
	CreateClient func() (thriftbp.Client, error)
}

// Close is nop and always returns nil error.
func (MockClientPool) Close() error {
	return nil
}

// IsExhausted returns Exhausted field.
func (m MockClientPool) IsExhausted() bool {
	return m.Exhausted
}

// Call implements TClient.
//
// If Exhausted is set to true,
// it returns clientpool.ErrExhausted as the error.
//
// If Exhausted is set to false,
// it creates a new client using the CreateClient field if it's set
// or uses default MockClient otherwise
// and uses that to implement Call.
func (m MockClientPool) Call(ctx context.Context, method string, args, result thrift.TStruct) (thrift.ResponseMeta, error) {
	client, err := m.getClient()
	if err != nil {
		return thrift.ResponseMeta{}, thriftbp.PoolError{Cause: err}
	}
	return client.Call(ctx, method, args, result)
}

func (m MockClientPool) getClient() (thriftbp.Client, error) {
	if m.Exhausted {
		return nil, clientpool.ErrExhausted
	}
	if m.CreateClient != nil {
		return m.CreateClient()
	}
	return &MockClient{}, nil
}

// CopyTStruct is a helper function that can be used to implement MockCall.
//
// In thrift.TClient and MockCall interfaces,
// the result is passed in as an arg to the function,
// so you can't directly assign a result to it.
// Instead, you'll need to use this helper function to copy a constructed result
// to the arg.
//
// Example:
//
//     myMockClient.AddMockCall(
//       "myEndpoint",
//       func(ctx context.Context, args, result thrift.TStruct) error {
//         return thrifttest.CopyTStruct(
//           ctx,
//           result,
//           &myservice.MyServiceMyEndpointResult{
//             Success: &myservice.MyEndpointResponse{
//               // Set the response fields.
//             },
//           },
//         )
//       },
//     )
func CopyTStruct(ctx context.Context, dst, src thrift.TStruct) error {
	proto := thrift.NewTBinaryProtocolConf(thrift.NewTMemoryBuffer(), nil)
	if err := src.Write(ctx, proto); err != nil {
		return err
	}
	return dst.Read(ctx, proto)
}

var (
	_ thrift.TClient      = (*MockClient)(nil)
	_ thrift.TClient      = (*RecordedClient)(nil)
	_ thriftbp.ClientPool = MockClientPool{}
	_ clientpool.Client   = (*MockClient)(nil)
)
