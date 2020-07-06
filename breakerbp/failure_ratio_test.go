package breakerbp_test

import (
	"context"
	"errors"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/breakerbp"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
)

var (
	testMinRequests      = 3
	testFailureThreshold = .5
	testMethod           = "method"
)

type testConfig struct {
	name         string
	shouldFail   bool
	numFailures  int
	numSuccesses int
}

var testCases = []testConfig{
	{
		name:         "no requests",
		shouldFail:   false,
		numFailures:  0,
		numSuccesses: 0,
	},
	{
		name:         "no failures",
		shouldFail:   false,
		numFailures:  0,
		numSuccesses: testMinRequests + 1,
	},
	{
		name:         "all failures",
		shouldFail:   true,
		numFailures:  testMinRequests + 1,
		numSuccesses: 0,
	},
	{
		name:         "too few requests",
		shouldFail:   false,
		numFailures:  testMinRequests - 1,
		numSuccesses: 0,
	},
	{
		name:         "low failure rate",
		shouldFail:   false,
		numFailures:  499,
		numSuccesses: 501, // 50.1% just above threshold.
	}}

func TestFailureBreaker(t *testing.T) {
	for _, c := range testCases {
		t.Run(c.name, c.run)
	}
}

func (config testConfig) run(t *testing.T) {
	cb := newTestCircuitBreaker()
	mock := &thrifttest.MockClient{
		FailUnregisteredMethods: true,
	}
	client := thrift.WrapClient(
		mock,
		cb.ThriftMiddleware,
	)
	mockRequests(config, mock, client)
	// reset client to succeed
	mock.AddNopMockCalls(testMethod)
	err := client.Call(context.Background(), testMethod, nil, nil)
	if err == nil && config.shouldFail {
		t.Errorf("test case {%v} expected to fail, but call returned without error", config.name)
	} else if err != nil && !config.shouldFail {
		t.Errorf("test case {%v} expected to succeed, but call returned error: %v", config.name, err)
	}
}

func mockRequests(config testConfig, mock *thrifttest.MockClient, client thrift.TClient) {
	mock.AddNopMockCalls(testMethod)
	for i := 0; i < config.numSuccesses; i++ {
		client.Call(context.Background(), testMethod, nil, nil)
	}
	mock.AddMockCall(testMethod,
		func(_ context.Context, args, result thrift.TStruct) error {
			return errors.New("backend down")
		})
	for i := 0; i < config.numFailures; i++ {
		client.Call(context.Background(), testMethod, nil, nil)
	}
}

func newTestCircuitBreaker() breakerbp.FailureRatioBreaker {
	config := breakerbp.Config{
		MinRequestsToTrip: testMinRequests,
		FailureThreshold:  testFailureThreshold,
	}
	return breakerbp.NewFailureRatioBreaker(config)
}
