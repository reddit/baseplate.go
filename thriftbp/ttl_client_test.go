package thriftbp

import (
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/reddit/baseplate.go/prometheusbp/promtest"
)

// firstSuccessGenerator is a ttlClientGenerator implementation that would
// return client and transport on the first call, and errors afterwards.
func firstSuccessGenerator(transport thrift.TTransport) ttlClientGenerator {
	factory := thrift.NewTBinaryProtocolFactoryConf(nil)
	client := thrift.NewTStandardClient(
		factory.GetProtocol(transport),
		factory.GetProtocol(transport),
	)
	first := true
	return func() (thrift.TClient, thrift.TTransport, error) {
		if first {
			first = false
			return client, transport, nil
		}
		return nil, nil, errors.New("error")
	}
}

func TestTTLClient(t *testing.T) {
	transport := thrift.NewTMemoryBuffer()
	ttl := time.Millisecond
	jitter := 0.1

	client, err := newTTLClient(firstSuccessGenerator(transport), ttl, jitter, "")
	if err != nil {
		t.Fatalf("newTTLClient returned error: %v", err)
	}
	if !client.IsOpen() {
		t.Error("Expected immediate IsOpen call to return true, got false.")
	}

	time.Sleep(ttl + time.Duration(float64(ttl)*jitter))
	if client.IsOpen() {
		t.Error("Expected IsOpen call after sleep to return false, got true.")
	}

	client, err = newTTLClient(firstSuccessGenerator(transport), ttl, -jitter, "")
	if err != nil {
		t.Fatalf("newTTLClient returned error: %v", err)
	}
	if !client.IsOpen() {
		t.Error("Expected immediate IsOpen call to return true, got false.")
	}

	time.Sleep(ttl)
	if client.IsOpen() {
		t.Error("Expected IsOpen call after sleep to return false, got true.")
	}
}

func TestTTLClientNegativeTTL(t *testing.T) {
	transport := thrift.NewTMemoryBuffer()
	ttl := time.Millisecond

	client, err := newTTLClient(firstSuccessGenerator(transport), -ttl, 0.1, "")
	if err != nil {
		t.Fatalf("newTTLClient returned error: %v", err)
	}
	if !client.IsOpen() {
		t.Error("Expected immediate IsOpen call to return true, got false.")
	}

	time.Sleep(ttl)
	if !client.IsOpen() {
		t.Error("Expected IsOpen call after sleep to return true, got false.")
	}
}

func TestTTLClientRenew(t *testing.T) {
	t.Run("no-ttl", func(t *testing.T) {
		c := &ttlClient{
			ttl: -1,
		}
		state := new(ttlClientState)
		state.renew(time.Now(), c)
		if !state.expiration.IsZero() {
			t.Errorf("Expected expiration to be zero with negative ttl, got %v", state.expiration)
		}
		if state.timer != nil {
			t.Errorf("Expected timer to be nil with negative ttl, got %#v", state.timer)
		}
	})
	t.Run("with-ttl", func(t *testing.T) {
		const ttl = time.Millisecond * 100
		c := &ttlClient{
			ttl: ttl,
		}
		state := new(ttlClientState)
		now := time.Now()
		state.renew(now, c)
		want := now.Add(ttl)
		if !state.expiration.Equal(want) {
			t.Errorf("expiration want %v, got %v", want, state.expiration)
		}
		if state.timer == nil {
			t.Fatal("timer is nil")
		}

		state.timer.Stop()
	})
}

// alwaysSuccessGenerator is a ttlClientGenerator implementation that would
// always return client, transport, and no error.
type alwaysSuccessGenerator struct {
	transport thrift.TTransport

	called atomic.Int64
}

func (g *alwaysSuccessGenerator) generator() ttlClientGenerator {
	factory := thrift.NewTBinaryProtocolFactoryConf(nil)
	client := thrift.NewTStandardClient(
		factory.GetProtocol(g.transport),
		factory.GetProtocol(g.transport),
	)
	return func() (thrift.TClient, thrift.TTransport, error) {
		g.called.Add(1)
		return client, g.transport, nil
	}
}

func (g *alwaysSuccessGenerator) numCalls() int64 {
	return g.called.Load()
}

type mockTTransport struct {
	thrift.TTransport

	closeCalled atomic.Int64
}

func (m *mockTTransport) Close() error {
	m.closeCalled.Add(1)
	return nil
}

func (m *mockTTransport) numCloses() int64 {
	return m.closeCalled.Load()
}

func TestTTLClientRefresh(t *testing.T) {
	t.Run("no-connection-leak", func(t *testing.T) {
		var transport mockTTransport
		const (
			buffer = time.Millisecond * 10
			ttl    = buffer * 5
			jitter = 0
		)

		g := alwaysSuccessGenerator{transport: &transport}
		client, err := newTTLClient(g.generator(), ttl, jitter, "" /* slug */)
		if err != nil {
			t.Fatalf("newTTLClient returned error: %v", err)
		}
		defer func() {
			state := <-client.state
			state.timer.Stop()
		}()

		if got := transport.numCloses(); got != 0 {
			t.Errorf("Expected transport.Close to be called 0 times, got %d", got)
		}

		time.Sleep(ttl + buffer)
		want := int64(1)
		if got := transport.numCloses(); got != want {
			t.Errorf("Expected transport.Close to be called %d time after sleep, got %d", want, got)
		}

		time.Sleep(ttl + buffer)
		want = 2
		if got := transport.numCloses(); got < want {
			t.Errorf("Expected transport.Close to be called at least %d time after second sleep, got %d", want, got)
		}
		// generator should always be called one more time than close
		want++
		if got := g.numCalls(); got < want {
			t.Errorf("Expected generator to be called at least %d time after second sleep, got %d", want, got)
		}
	})

	t.Run("retry", func(t *testing.T) {
		// This test is to make sure that closing the ttlClient will also stop its
		// retry attempts.
		const slug = "slug"
		g := alwaysSuccessGenerator{transport: &mockTTransport{}}
		client, err := newTTLClient(g.generator(), 0 /* ttl */, 0 /* jitter */, slug)
		if err != nil {
			t.Fatalf("newTTLClient returned error: %v", err)
		}
		// Replace generator to something that would always fail to test retries
		client.generator = func() (thrift.TClient, thrift.TTransport, error) {
			// The error must be of type net.Error to trigger retry
			return nil, nil, &net.OpError{
				Err: errors.New("failed"),
			}
		}

		const (
			buffer = 50 * time.Millisecond
			// It should have made 2 attempts after this sleep (initial + first retry)
			sleep = ttlClientRefreshInitialDelay + ttlClientRefreshMaxJitter + buffer
		)

		defer promtest.NewPrometheusMetricTest(t, "failed refreshes", ttlClientReplaceCounter, prometheus.Labels{
			clientNameLabel: slug,
			successLabel:    "false",
		}).CheckDelta(2)

		defer promtest.NewPrometheusMetricTest(t, "refresh attempts histogram", ttlClientRefreshAttemptsHisto, prometheus.Labels{
			clientNameLabel: slug,
		}).CheckSampleCountDelta(1)

		go func() {
			time.Sleep(sleep)
			client.Close()
		}()
		before := time.Now()
		client.refresh()
		elapsed := time.Since(before)
		if elapsed < sleep || elapsed > sleep+buffer {
			t.Errorf("client.refresh() took %v, want %v", elapsed, sleep)
		}
	})
}
