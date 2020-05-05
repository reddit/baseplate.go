package baseplate_test

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	baseplate "github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/metricsbp"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/tracing"
)

// pubkey copied from https://github.com/reddit/baseplate.py/blob/db9c1d7cddb1cb242546349e821cad0b0cbd6fce/tests/__init__.py#L12
const (
	secretStore = `{
	"secrets": {
		"secret/authentication/public-key": {
			"type": "versioned",
			"current": "foobar",
			"previous": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtzMnDEQPd75QZByogNlB\nNY2auyr4sy8UNTDARs79Edq/Jw5tb7ub412mOB61mVrcuFZW6xfmCRt0ILgoaT66\nTp1RpuEfghD+e7bYZ+Q2pckC1ZaVPIVVf/ZcCZ0tKQHoD8EpyyFINKjCh516VrCx\nKuOm2fALPB/xDwDBEdeVJlh5/3HHP2V35scdvDRkvr2qkcvhzoy0+7wUWFRZ2n6H\nTFrxMHQoHg0tutAJEkjsMw9xfN7V07c952SHNRZvu80V5EEpnKw/iYKXUjCmoXm8\ntpJv5kXH6XPgfvOirSbTfuo+0VGqVIx9gcomzJ0I5WfGTD22dAxDiRT7q7KZnNgt\nTwIDAQAB\n-----END PUBLIC KEY-----"
		},
		"secret/http/edge-context-signature": {
			"type": "versioned",
			"current": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU=",
			"previous": "aHVudGVyMg==",
			"encoding": "base64"
		},
		"secret/http/span-signature": {
			"type": "versioned",
			"current": "Y2RvVXhNMVdsTXJma3BDaHRGZ0dPYkVGSg==",
			"encoding": "base64"
		}
	},
	"vault": {
		"url": "vault.reddit.ue1.snooguts.net",
		"token": "17213328-36d4-11e7-8459-525400f56d04"
	}
}`

	testTimeout = time.Millisecond * 100
)

func newSecretsStore(t testing.TB) (store *secrets.Store, dir string) {
	t.Helper()

	dir, err := ioutil.TempDir("", "edge_context_test_")
	if err != nil {
		t.Fatal(err)
	}

	tmpFile, err := ioutil.TempFile(dir, "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write([]byte(secretStore)); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = secrets.NewStore(context.Background(), tmpPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	return
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

	store, dir := newSecretsStore(t)
	defer func() {
		os.RemoveAll(dir)
		store.Close()
	}()

	bp := baseplate.NewTestBaseplate(baseplate.Config{StopTimeout: testTimeout}, store)
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
			server:      newWaitServer(t, bp, bp.Config().StopTimeout*2),
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
					ch <- baseplate.Serve(context.Background(), c.server)
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

func float64Ptr(v float64) *float64 {
	return &v
}

type serviceConfig struct {
	Redis struct {
		Addrs []string
	}
}

func TestDecodeConfigYAML(t *testing.T) {
	const raw = `
addr: :8080
timeout: 30s
stopTimeout: 30s

log:
 level: info

metrics:
 namespace: baseplate-test
 endpoint: metrics:8125
 histogramSampleRate: 0.01

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

	expected := baseplate.Config{
		Addr:        ":8080",
		Timeout:     time.Second * 30,
		StopTimeout: time.Second * 30,

		Log: log.Config{
			Level: "info",
		},

		Metrics: metricsbp.Config{
			Namespace:           "baseplate-test",
			Endpoint:            "metrics:8125",
			CounterSampleRate:   nil,
			HistogramSampleRate: float64Ptr(0.01),
		},

		Secrets: secrets.Config{
			Path: "/tmp/secrets.json",
		},

		Tracing: tracing.Config{
			Namespace:     "baseplate-test",
			QueueName:     "test",
			RecordTimeout: time.Millisecond,
			SampleRate:    0.01,
		},
	}

	expectedServiceCfg := serviceConfig{
		struct{ Addrs []string }{
			Addrs: []string{
				"redis:8000",
				"redis:8001",
			},
		},
	}
	var serviceCfg serviceConfig
	cfg, err := baseplate.DecodeConfigYAML(strings.NewReader(raw), &serviceCfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, expected) {
		t.Fatalf("config mismatch, expected %#v, got %#v", expected, cfg)
	}
	if !reflect.DeepEqual(serviceCfg, expectedServiceCfg) {
		t.Fatalf(
			"service config mismatch, expected %#v, got %#v",
			expectedServiceCfg,
			serviceCfg,
		)
	}
}
