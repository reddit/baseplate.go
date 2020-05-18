package edgecontext_test

import (
	"context"
	"os"
	"testing"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
)

var globalTestImpl *edgecontext.Impl

func TestMain(m *testing.M) {
	store, _, err := secrets.NewTestSecrets(
		context.Background(),
		make(map[string]secrets.GenericSecret),
	)
	if err != nil {
		log.Panic(err)
	}
	defer store.Close()

	globalTestImpl = edgecontext.Init(edgecontext.Config{Store: store})
	os.Exit(m.Run())
}
