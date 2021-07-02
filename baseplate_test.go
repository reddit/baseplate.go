package baseplate_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/configbp"
	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/runtimebp"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	testTimeout = time.Millisecond * 100
)

func newSecretsStore(t testing.TB) *secrets.Store {
	t.Helper()

	store, _, err := secrets.NewTestSecrets(
		context.Background(),
		make(map[string]secrets.GenericSecret),
	)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func newWaitServer(t testing.TB, bp baseplate.Baseplate, duration time.Duration) baseplate.Server {
	t.Helper()

	wg := sync.WaitGroup{}
	wg.Add(1)
	return &testServer{
		bp:           bp,
		waitDuration: duration,
		wg:           &wg,
	}
}

func newErrorServer(t testing.TB, bp baseplate.Baseplate, closeErr error) baseplate.Server {
	t.Helper()

	wg := sync.WaitGroup{}
	wg.Add(1)
	return &testServer{
		bp:       bp,
		closeErr: closeErr,
		wg:       &wg,
	}
}

type testServer struct {
	bp           baseplate.Baseplate
	closeErr     error
	waitDuration time.Duration
	wg           *sync.WaitGroup
}

func (s *testServer) Baseplate() baseplate.Baseplate {
	return s.bp
}

func (s *testServer) Serve() error {
	s.wg.Wait()
	return nil
}

func (s *testServer) Close() error {
	if s.waitDuration != 0 {
		time.Sleep(s.waitDuration)
	}
	s.wg.Done()
	return s.closeErr
}

var _ baseplate.Server = (*testServer)(nil)

func TestServe(t *testing.T) {
	t.Parallel()

	store := newSecretsStore(t)
	defer store.Close()

	bp := baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
		Config:          baseplate.Config{StopTimeout: testTimeout},
		Store:           store,
		EdgeContextImpl: ecinterface.Mock(),
	})
	closeError := errors.New("test close error")

	cases := []struct {
		name        string
		server      baseplate.Server
		errExpected error
	}{
		{
			name:        "fast",
			server:      newWaitServer(t, bp, time.Millisecond),
			errExpected: nil,
		},
		{
			name:        "timeout",
			server:      newWaitServer(t, bp, bp.GetConfig().StopTimeout*2),
			errExpected: context.DeadlineExceeded,
		},
		{
			name:        "close-error",
			server:      newErrorServer(t, bp, closeError),
			errExpected: closeError,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				ch := make(chan error)

				go func() {
					// Run Serve in a goroutine since it is blocking
					ch <- baseplate.Serve(
						context.Background(),
						baseplate.ServeArgs{Server: c.server},
					)
				}()

				time.Sleep(time.Millisecond)
				p, err := os.FindProcess(syscall.Getpid())
				if err != nil {
					t.Fatal(err)
				}

				p.Signal(os.Interrupt)
				err = <-ch

				if !errors.Is(err, c.errExpected) {
					t.Fatalf("error mismatch, expected %#v, got %#v", c.errExpected, err)
				}
			},
		)
	}
}

type timestampCloser struct {
	ts []time.Time
}

func (c *timestampCloser) Close() error {
	c.ts = append(c.ts, time.Now())
	return nil
}

func TestServeClosers(t *testing.T) {
	t.Parallel()

	store := newSecretsStore(t)
	defer store.Close()

	bp := baseplate.NewTestBaseplate(baseplate.NewTestBaseplateArgs{
		Config:          baseplate.Config{StopTimeout: testTimeout},
		Store:           store,
		EdgeContextImpl: ecinterface.Mock(),
	})

	pre := &timestampCloser{}
	post := &timestampCloser{}

	args := baseplate.ServeArgs{
		Server:       newWaitServer(t, bp, time.Millisecond),
		PreShutdown:  []io.Closer{pre},
		PostShutdown: []io.Closer{post},
	}

	ch := make(chan error)

	p, err := os.FindProcess(syscall.Getpid())
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		// Run Serve in a goroutine since it is blocking
		ch <- baseplate.Serve(context.Background(), args)
	}()

	time.Sleep(time.Millisecond)
	p.Signal(os.Interrupt)
	<-ch

	if len(pre.ts) != 1 {
		t.Fatalf("Unexpected number of PreShutdown calls: expected 1, got %v", len(pre.ts))
	}
	if len(post.ts) != 1 {
		t.Fatalf("Unexpected number of PostShutdown calls: expected 1, got %v", len(post.ts))
	}

	if !pre.ts[0].Before(post.ts[0]) {
		t.Errorf(
			"PreShutdown finished after PostShutdown: pre: %v, post: %v",
			pre.ts[0],
			post.ts[0],
		)
	}
}

func float64Ptr(v float64) *float64 {
	return &v
}

type serviceConfig struct {
	baseplate.Config `yaml:",inline"`

	Redis struct {
		Addrs []string
	} `yaml:"redis"`
}

func TestParseConfigYAML(t *testing.T) {
	t.Cleanup(func() { configbp.BaseplateConfigPath = os.Getenv("BASEPLATE_CONFIG_PATH") })
	useConfig := func(configYAML string) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(configYAML), 0666); err != nil {
			t.Fatalf("Failed to write to tmp config file %q: %v", path, err)
		}
		configbp.BaseplateConfigPath = path
	}

	const validConfigYAML = `
addr: :8080
timeout: 30s
stopTimeout: 30s

log:
 level: info

metrics:
 namespace: baseplate-test
 endpoint: metrics:8125
 histogramSampleRate: 0.01

runtime:
 numProcesses:
  max: 100

secrets:
 path: /tmp/secrets.json

tracing:
 namespace: baseplate-test
 queueName: test
 recordTimeout: 1ms
 sampleRate: 0.01

redis:
 addrs:
  - redis:8000
  - redis:8001
`
	validConfigBaseplate := baseplate.Config{
		Addr:        ":8080",
		Timeout:     time.Second * 30,
		StopTimeout: time.Second * 30,

		Log: log.Config{
			Level: "info",
		},

		Metrics: metricsbp.Config{
			Namespace:           "baseplate-test",
			Endpoint:            "metrics:8125",
			HistogramSampleRate: float64Ptr(0.01),
		},

		Runtime: runtimebp.Config{
			NumProcesses: struct {
				Max int `yaml:"max"`
				Min int `yaml:"min"`
			}{
				Max: 100,
				Min: 0,
			},
		},

		Secrets: secrets.Config{
			Path: "/tmp/secrets.json",
		},

		Tracing: tracing.Config{
			Namespace:        "baseplate-test",
			QueueName:        "test",
			MaxRecordTimeout: time.Millisecond,
			SampleRate:       0.01,
		},
	}

	validConfigService := struct{ Addrs []string }{
		Addrs: []string{
			"redis:8000",
			"redis:8001",
		},
	}

	t.Run("valid_config", func(t *testing.T) {
		useConfig(validConfigYAML)
		var cfg serviceConfig
		if err := baseplate.ParseConfigYAML(&cfg); err != nil {
			t.Fatalf("valid config failed to parse: %s", err)
		}
		if !reflect.DeepEqual(cfg.GetConfig(), validConfigBaseplate) {
			t.Errorf("config mismatch, expected %#v, got %#v", validConfigBaseplate, cfg.GetConfig())
		}
		if !reflect.DeepEqual(cfg.Redis, validConfigService) {
			t.Errorf(
				"service config mismatch, expected %#v, got %#v",
				validConfigService,
				cfg.Redis,
			)
		}
	})

	t.Run("extra_params", func(t *testing.T) {
		const configWithExtraParams = validConfigYAML + `
anotherbackend:
  addr: someservice:9090
`
		useConfig(configWithExtraParams)
		var cfg serviceConfig
		if err := baseplate.ParseConfigYAML(&cfg); err == nil {
			t.Error("Expected error when yaml has extra content, did not happen.")
		}
	})
}
