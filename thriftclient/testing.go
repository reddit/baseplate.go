package thriftclient

import (
	"context"

	"github.com/apache/thrift/lib/go/thrift"
)

// MockCall is a mock function that can be registered to a method in a MockClient
type MockCall func(ctx context.Context, args, result thrift.TStruct) error

// MockClient implements thrift.TClient and can be used to mock out thrift calls
// in testing by using it as the base client rather than a real client.
//
// If a MockCall is registered to a method, then Call will return the result of
// that MockCall when that method is Call-ed.  If no MockCall is registered to a
// method, Call will simply return nil.
//
// MockClient is provided to help with unit testing and should not be used in
// production code.
type MockClient struct {
	methods map[string]MockCall
}

// AddMockCall registers the given MockCall to the given method.
//
// If a mock is already registered to that method, it will be replaced with the
// new mock.
//
// AddMockCall is not thread-safe.
func (c *MockClient) AddMockCall(method string, mock MockCall) {
	if c.methods == nil {
		c.methods = make(map[string]MockCall)
	}
	c.methods[method] = mock
}

// Call fufills the thrift.TClient interface. It will return the result of the
// MockCall registered to method if one exists, otherwise it returns nil.
func (c *MockClient) Call(ctx context.Context, method string, args, result thrift.TStruct) error {
	if m, ok := c.methods[method]; ok {
		return m(ctx, args, result)
	}
	return nil
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

var (
	_ thrift.TClient = (*MockClient)(nil)
	_ thrift.TClient = (*RecordedClient)(nil)
)
