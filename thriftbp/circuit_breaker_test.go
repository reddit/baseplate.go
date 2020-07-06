package thriftbp_test

import (
	"context"
	"errors"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/sony/gobreaker"

	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
)

var (
	testMinRequests      = 3
	testFailureThreshold = .5
	testMethod           = "method"
)

func TestShouldTrip(t *testing.T) {
	cb := newTestCircuitBreaker()
	counts := gobreaker.Counts{
		Requests:      uint32(testMinRequests + 1),
		TotalFailures: 0,
	}
	tripped := cb.ShouldTripCircuitBreaker(counts)
	if tripped {
		t.Errorf("expected circuit breaker not to trip but it did")
	}
}

func TestShouldTrip_zeroRequests(t *testing.T) {
	cb := newTestCircuitBreaker()
	counts := gobreaker.Counts{
		Requests:      0,
		TotalFailures: 0,
	}
	tripped := cb.ShouldTripCircuitBreaker(counts)
	if !tripped {
		t.Errorf("expected circuit breaker not to trip but it did")
	}
}

func TestShouldTrip_failures(t *testing.T) {
	cb := newTestCircuitBreaker()
	totalRequests := uint32(testMinRequests + 1)
	counts := gobreaker.Counts{
		Requests:      totalRequests,
		TotalFailures: totalRequests,
	}
	tripped := cb.ShouldTripCircuitBreaker(counts)
	if tripped {
		t.Errorf("expected circuit breaker to trip but it did not")
	}
}

func TestShouldTrip_tooFewRequests(t *testing.T) {
	cb := newTestCircuitBreaker()
	totalRequests := uint32(testMinRequests - 1)
	counts := gobreaker.Counts{
		Requests:      totalRequests,
		TotalFailures: totalRequests,
	}
	tripped := cb.ShouldTripCircuitBreaker(counts)
	if tripped {
		t.Errorf("expected circuit breaker not to trip but it did")
	}
}

func TestShouldTrip_lowFaiureRate(t *testing.T) {
	cb := newTestCircuitBreaker()
	counts := gobreaker.Counts{
		Requests:      1000,
		TotalFailures: 499, // just below .5 rate
	}
	tripped := cb.ShouldTripCircuitBreaker(counts)
	if tripped {
		t.Errorf("expected circuit breaker not to trip but it did")
	}
}

func TestThriftMiddleware(t *testing.T) {
	cb := newTestCircuitBreaker()
	mock := &thrifttest.MockClient{
		FailUnregisteredMethods: true,
	}
	client := thrift.WrapClient(
		mock,
		cb.ThriftMiddleware,
	)
	mock.AddNopMockCalls(testMethod)
	err := client.Call(context.Background(), testMethod, nil, nil)
	if err != nil {
		t.Errorf("expected no error but got %v", err)
	}
	// fail calls to trip circuit breaker.
	mock.AddMockCall(testMethod,
		func(_ context.Context, args, result thrift.TStruct) error {
			return errors.New("backend down")
		})
	for i := 0; i < testMinRequests+1; i++ {
		err := client.Call(context.Background(), testMethod, nil, nil)
		if err == nil {
			t.Errorf("expected an error, but got none")
		}
	}
	// reset client so it succeeds. call should still fail due to cb
	mock.AddNopMockCalls(testMethod)
	err = client.Call(context.Background(), testMethod, nil, nil)
	if err != nil {
		t.Errorf("expected no error but got %v", err)
	}
}

func newTestCircuitBreaker() thriftbp.FailureRatioBreaker {
	config := thriftbp.CircuitBreakerConfig{
		MinRequestsToTrip: testMinRequests,
		FailureThreshold:  testFailureThreshold,
	}
	return thriftbp.NewFailureRatioBreaker(config)
}
