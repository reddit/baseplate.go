package thrifttest_test

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/reddit/baseplate.go/batcherror"
	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
)

// pubkey copied from https://github.com/reddit/baseplate.py/blob/db9c1d7cddb1cb242546349e821cad0b0cbd6fce/tests/__init__.py#L12
const secretStore = `
{
	"secrets": {
		"secret/authentication/public-key": {
			"type": "versioned",
			"current": "foobar",
			"previous": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtzMnDEQPd75QZByogNlB\nNY2auyr4sy8UNTDARs79Edq/Jw5tb7ub412mOB61mVrcuFZW6xfmCRt0ILgoaT66\nTp1RpuEfghD+e7bYZ+Q2pckC1ZaVPIVVf/ZcCZ0tKQHoD8EpyyFINKjCh516VrCx\nKuOm2fALPB/xDwDBEdeVJlh5/3HHP2V35scdvDRkvr2qkcvhzoy0+7wUWFRZ2n6H\nTFrxMHQoHg0tutAJEkjsMw9xfN7V07c952SHNRZvu80V5EEpnKw/iYKXUjCmoXm8\ntpJv5kXH6XPgfvOirSbTfuo+0VGqVIx9gcomzJ0I5WfGTD22dAxDiRT7q7KZnNgt\nTwIDAQAB\n-----END PUBLIC KEY-----"
		}
	}
}`

type BaseplateService struct {
	Fail bool
	Err  error
}

func (srv BaseplateService) IsHealthy(ctx context.Context) (r bool, err error) {
	return !srv.Fail, srv.Err
}

type Closer struct {
	Closers []func() error
}

func (c *Closer) Add(closer func() error) {
	c.Closers = append(c.Closers, closer)
}

func (c *Closer) Close() error {
	var errs batcherror.BatchError
	for _, closer := range c.Closers {
		if err := closer(); err != nil {
			errs.Add(err)
		}
	}
	return errs.Compile()
}

func newSecrets(t testing.TB) (*secrets.Store, io.Closer) {
	t.Helper()

	closer := &Closer{}
	dir, err := ioutil.TempDir("", "thifttest-server-test-")
	if err != nil {
		t.Fatal(err)
	}
	closer.Add(func() error {
		return os.RemoveAll(dir)
	})

	tmpFile, err := ioutil.TempFile(dir, "secrets.json")
	if err != nil {
		closer.Close()
		t.Fatal(err)
	}

	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write([]byte(secretStore)); err != nil {
		closer.Close()
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		closer.Close()
		t.Fatal(err)
	}

	store, err := secrets.NewStore(context.Background(), tmpPath, nil)
	if err != nil {
		closer.Close()
		t.Fatal(err)
	}
	closer.Add(store.Close)
	return store, closer
}

func TestNewBaseplateServer(t *testing.T) {
	store, closer := newSecrets(t)
	defer closer.Close()

	testErr := errors.New("test")

	type expectation struct {
		result bool
		hasErr bool
	}
	cases := []struct {
		name     string
		handler  BaseplateService
		expected expectation
	}{
		{
			name:    "success/no-err",
			handler: BaseplateService{},
			expected: expectation{
				result: true,
				hasErr: false,
			},
		},
		{
			name:    "fail/no-err",
			handler: BaseplateService{Fail: true},
			expected: expectation{
				result: false,
				hasErr: false,
			},
		},
		{
			name:    "fail/err",
			handler: BaseplateService{Fail: true, Err: testErr},
			expected: expectation{
				result: false,
				hasErr: true,
			},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				processor := baseplatethrift.NewBaseplateServiceProcessor(c.handler)
				server, err := thrifttest.NewBaseplateServer(store, processor, nil)
				if err != nil {
					t.Fatal(err)
				}
				// cancelling the context will close the server.
				server.Start(ctx)

				client := baseplatethrift.NewBaseplateServiceClient(server.ClientPool)
				result, err := client.IsHealthy(ctx)

				if c.expected.hasErr && err == nil {
					t.Error("expected an error, got nil")
				} else if !c.expected.hasErr && err != nil {
					t.Errorf("expected no error, got %v", err)
				}

				if result != c.expected.result {
					t.Errorf("result mismatch, expected %v, got %v", c.expected.result, result)
				}
			},
		)
	}
}
