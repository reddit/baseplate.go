package log

import (
	"context"
	"errors"
	"testing"

	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
)

func TestZapLogger(t *testing.T) {
	InitLogger(DebugLevel)
	log(internalv2compat.GlobalLogger())

	Version = "test-version"
	InitLoggerJSON(DebugLevel)
	log(internalv2compat.GlobalLogger())
}

func TestInitSentry(t *testing.T) {
	c, err := InitSentry(SentryConfig{})
	if err != nil {
		t.Fatal(err)
	}
	ErrorWithSentry(context.Background(), "", errors.New("what"))
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}
