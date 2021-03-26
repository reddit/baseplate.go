package healthcheck

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/thriftbp"
)

type healthyMap = map[baseplate.IsHealthyProbe]bool

var (
	allHealthy = healthyMap{
		baseplate.IsHealthyProbe_READINESS: true,
		baseplate.IsHealthyProbe_LIVENESS:  true,
		baseplate.IsHealthyProbe_STARTUP:   true,
	}
	allUnhealthy = healthyMap(nil)
)

type service struct {
	addr string
	up   func(t *testing.T)
	down func(t *testing.T)
}

type thriftHandler struct {
	healthy healthyMap
}

func (th thriftHandler) IsHealthy(ctx context.Context, req *baseplate.IsHealthyRequest) (bool, error) {
	return th.healthy[req.GetProbe()], nil
}

func thriftService(healthy healthyMap) *service {
	var wg sync.WaitGroup
	var server *thrift.TSimpleServer

	s := new(service)
	s.up = func(t *testing.T) {
		t.Helper()

		socket, err := thrift.NewTServerSocket("localhost:0")
		if err != nil {
			t.Fatalf("Failed to create service socket: %v", err)
		}
		if err := socket.Listen(); err != nil {
			t.Fatalf("Failed to start listener: %v", err)
		}
		s.addr = socket.Addr().String()
		t.Logf("Listening on %v...", s.addr)
		server, err = thriftbp.NewServer(thriftbp.ServerConfig{
			Socket: socket,
			Processor: baseplate.NewBaseplateServiceV2Processor(&thriftHandler{
				healthy: healthy,
			}),
		})
		if err != nil {
			t.Fatalf("Failed to start service: %v", err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := server.Serve(); err != nil {
				t.Errorf("server.Serve returned error: %v", err)
			}
		}()
	}
	s.down = func(t *testing.T) {
		t.Helper()

		if server != nil {
			if err := server.Stop(); err != nil {
				t.Errorf("Failed to stop service: %v", err)
			}
		}
		wg.Wait()
	}
	return s
}

func httpService(healthy healthyMap) *service {
	var wg sync.WaitGroup
	mux := http.NewServeMux()
	server := &http.Server{
		Handler: mux,
	}

	s := new(service)
	s.up = func(t *testing.T) {
		t.Helper()

		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("Failed to create listener: %v", err)
		}
		s.addr = listener.Addr().String()
		t.Logf("Listening on %v...", s.addr)
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			probe, err := httpbp.GetHealthCheckProbe(r.URL.Query())
			if err != nil {
				t.Errorf("httpbp.GetHealthCheckProbe returned error: %v", err)
			}
			if healthy[baseplate.IsHealthyProbe(probe)] {
				io.WriteString(w, "ok")
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, "not ok")
			}
		})
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				t.Errorf("server.Serve returned error: %v", err)
			}
		}()
	}
	s.down = func(t *testing.T) {
		t.Helper()

		if err := server.Shutdown(context.Background()); err != nil {
			t.Errorf("Failed to shutdown http server: %v", err)
		}
		wg.Wait()
	}
	return s
}

func TestRunArgs(t *testing.T) {
	const timeout = time.Millisecond * 100
	for _, c := range []struct {
		label   string
		args    []string
		err     bool
		service *service
	}{
		{
			label:   "default",
			service: thriftService(allHealthy),
		},
		{
			label:   "all-unhealthy-thrift",
			err:     true,
			service: thriftService(allUnhealthy),
		},
		{
			label: "startup-unhealthy-thrift-1",
			args:  []string{"--type", "thrift", "--probe", "startup"},
			err:   true,
			service: thriftService(healthyMap{
				baseplate.IsHealthyProbe_READINESS: true,
				baseplate.IsHealthyProbe_LIVENESS:  true,
				baseplate.IsHealthyProbe_STARTUP:   false,
			}),
		},
		{
			label: "startup-unhealthy-thrift-2",
			err:   false, // This one checks readiness probe so it should report healthy
			service: thriftService(healthyMap{
				baseplate.IsHealthyProbe_READINESS: true,
				baseplate.IsHealthyProbe_LIVENESS:  false,
				baseplate.IsHealthyProbe_STARTUP:   false,
			}),
		},
		{
			label:   "http",
			args:    []string{"--type", "wsgi"},
			service: httpService(allHealthy),
		},
		{
			label:   "all-unhealthy-http",
			args:    []string{"--type", "wsgi"},
			err:     true,
			service: httpService(allUnhealthy),
		},
		{
			label: "liveness-unhealthy-http-1",
			args:  []string{"--type", "wsgi", "--probe", "liveness"},
			err:   true,
			service: httpService(healthyMap{
				baseplate.IsHealthyProbe_READINESS: true,
				baseplate.IsHealthyProbe_LIVENESS:  false,
				baseplate.IsHealthyProbe_STARTUP:   true,
			}),
		},
		{
			label: "liveness-unhealthy-http-2",
			args:  []string{"--type", "wsgi"},
			err:   false, // This one checks readiness probe so it should report healthy
			service: httpService(healthyMap{
				baseplate.IsHealthyProbe_READINESS: true,
				baseplate.IsHealthyProbe_LIVENESS:  false,
				baseplate.IsHealthyProbe_STARTUP:   false,
			}),
		},
		{
			label: "help",
			args:  []string{"-h"},
			err:   true,
		},
		{
			label: "unknown-flag",
			args:  []string{"--fancy"},
			err:   true,
		},
		{
			label: "wrong-type",
			args:  []string{"--type", "foo"},
			err:   true,
		},
		{
			label: "wrong-probe",
			args:  []string{"--probe", "bar"},
			err:   true,
		},
		{
			label: "wrong-endpoint",
			args:  []string{"--endpoint", "localhost:1"},
			err:   true,
		},
		{
			label:   "short-timeout",
			args:    []string{"--timeout", "1ns"},
			err:     true,
			service: thriftService(allHealthy),
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			args := []string{"./healthcheck", "--timeout", timeout.String()}

			if c.service != nil {
				if c.service.up != nil {
					c.service.up(t)
				}
				if c.service.down != nil {
					t.Cleanup(func() {
						c.service.down(t)
					})
				}
				if c.service.addr != "" {
					args = append(args, "--endpoint", c.service.addr)
				}
			}
			args = append(args, c.args...)

			err := runArgs(args, io.Discard)
			if err != nil {
				t.Logf("error: %v", err)
			}
			if c.err {
				if err == nil {
					t.Error("Expected error, got none")
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect error, got: %v", err)
				}
			}
		})
	}
}

func TestRunArgsPositional(t *testing.T) {
	const timeout = time.Millisecond * 100

	t.Run("normal", func(t *testing.T) {
		service := thriftService(allHealthy)
		service.up(t)
		t.Cleanup(func() {
			service.down(t)
		})

		args := []string{"./healthcheck", "--timeout", timeout.String(), service.addr}

		err := runArgs(args, io.Discard)
		if err != nil {
			t.Errorf("Did not expect error, got: %v", err)
		}
	})

	t.Run("override", func(t *testing.T) {
		service := thriftService(allHealthy)
		service.up(t)
		t.Cleanup(func() {
			service.down(t)
		})

		args := []string{"./healthcheck", "--timeout", timeout.String(), "--type", "http", "thrift", service.addr}

		err := runArgs(args, io.Discard)
		if err != nil {
			t.Errorf("Did not expect error, got: %v", err)
		}
	})

	t.Run("wrong-type", func(t *testing.T) {
		service := thriftService(allHealthy)
		service.up(t)
		t.Cleanup(func() {
			service.down(t)
		})

		args := []string{"./healthcheck", "fancy", service.addr}

		err := runArgs(args, io.Discard)
		if err == nil {
			t.Error("Expected error, got none")
		}
	})

	t.Run("more-than-3", func(t *testing.T) {
		service := thriftService(allHealthy)
		service.up(t)
		t.Cleanup(func() {
			service.down(t)
		})

		args := []string{"./healthcheck", "thrift", service.addr, "foo"}

		err := runArgs(args, io.Discard)
		if err == nil {
			t.Error("Expected error, got none")
		}
	})
}
