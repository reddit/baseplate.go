package edgecontext_test

import (
	"context"
	"os"
	"testing"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
)

var (
	globalSecretsStore *secrets.Store
	globalTestImpl     *edgecontext.Impl
)

func TestMain(m *testing.M) {
	var err error
	globalSecretsStore, _, err = secrets.NewTestSecrets(
		context.Background(),
		make(map[string]secrets.GenericSecret),
	)
	if err != nil {
		log.Panic(err)
	}
	defer globalSecretsStore.Close()

	globalTestImpl = edgecontext.Init(edgecontext.Config{Store: globalSecretsStore})
	os.Exit(m.Run())
}
