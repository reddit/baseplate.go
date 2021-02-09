package thriftbp_test

import (
	"context"
	"testing"

	"github.com/reddit/baseplate.go/secrets"
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
