package thriftclient

import (
	"context"
	"errors"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/clientpool"
)

// MockCall is a mock function that can be registered to a method in a MockClient
type MockCall func(ctx context.Context, args, result thrift.TStruct) error

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

func nopMockCall(ctx context.Context, args, result thrift.TStruct) error {
	return nil
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
func (c *MockClient) Call(ctx context.Context, method string, args, result thrift.TStruct) error {
	if m, ok := c.methods[method]; ok {
		return m(ctx, args, result)
	}
	if c.FailUnregisteredMethods {
		return errors.New("thriftclient.MockClient: unregistered method: " + method)
	}
	return nil
}

// Close implements Client and is a nop that always returns nil.
func (MockClient) Close() error {
	return nil
}

// IsOpen implements Client and is a nop that always returns true.
func (MockClient) IsOpen() bool {
	return true
}

// Call records the inputs passed to RecordedClient.Call.
type Call struct {
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

	calls []Call
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
func (c RecordedClient) Calls() []Call {
	calls := make([]Call, len(c.calls))
	copy(calls, c.calls)
	return calls
}

// Call fufills the thrift.TClient interface. It will record the inputs to Call
// and either return the result of the inner client.Call or nil if the inner
// client is nil.
func (c *RecordedClient) Call(ctx context.Context, method string, args, result thrift.TStruct) error {
	c.calls = append(c.calls, Call{
		Ctx:    ctx,
		Method: method,
		args:   args,
		result: result,
	})
	if c.client != nil {
		return c.client.Call(ctx, method, args, result)
	}
	return nil
}

// MockClientPool is a ClientPool implementation can be used in test code.
type MockClientPool struct {
	Exhausted    bool
	CreateClient func() (Client, error)
}

// Close is nop and always returns nil error.
func (MockClientPool) Close() error {
	return nil
}

// IsExhausted returns Exhausted field.
func (m MockClientPool) IsExhausted() bool {
	return m.Exhausted
}

// GetClient implements ClientPool interface.
//
// If Exhausted is set to true,
// it returns clientpool.ErrExhausted as the error.
//
// If Exhausted is set to false,
// it calls CreateClient field if it's set,
// or return a default MockClient otherwise.
func (m MockClientPool) GetClient() (Client, error) {
	if m.Exhausted {
		return nil, clientpool.ErrExhausted
	}
	if m.CreateClient != nil {
		return m.CreateClient()
	}
	return &MockClient{}, nil
}

// ReleaseClient is nop.
func (MockClientPool) ReleaseClient(Client) {}

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
//         return thriftclient.CopyTStruct(
//           &myservice.MyServiceMyEndpointResult{
//             Success: &myservice.MyEndpointResponse{
//               // Set the response fields.
//             },
//           },
//           result,
//         )
//       },
//     )
func CopyTStruct(from, to thrift.TStruct) error {
	proto := thrift.NewTBinaryProtocolTransport(thrift.NewTMemoryBuffer())
	if err := from.Write(proto); err != nil {
		return err
	}
	return to.Read(proto)
}

var (
	_ thrift.TClient = (*MockClient)(nil)
	_ thrift.TClient = (*RecordedClient)(nil)
	_ Client         = (*MockClient)(nil)
	_ ClientPool     = MockClientPool{}
)
