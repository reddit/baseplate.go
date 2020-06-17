package thrifttest_test

import (
	"context"
	"errors"
	"testing"

	baseplatethrift "github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/thriftbp/thrifttest"
)

type BaseplateService struct {
	Fail bool
	Err  error
}

func (srv BaseplateService) IsHealthy(ctx context.Context) (r bool, err error) {
	return !srv.Fail, srv.Err
}

func newSecrets(t testing.TB) *secrets.Store {
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

func TestNewBaseplateServer(t *testing.T) {
	store := newSecrets(t)
	defer store.Close()

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
				server, err := thrifttest.NewBaseplateServer(thrifttest.ServerConfig{
					Processor:   processor,
					SecretStore: store,
				})
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
