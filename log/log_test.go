package log

import (
	"context"
	"errors"
	"testing"
)

func TestZapLogger(t *testing.T) {
	InitLogger(DebugLevel)
	log(globalLogger)

	Version = "test-version"
	InitLoggerJSON(DebugLevel)
	log(globalLogger)
}

func TestInitSentry(t *testing.T) {
	c, err := InitSentry(SentryConfig{})
	if err != nil {
		t.Fatal(err)
	}
	ErrorWithSentry(context.Background(), "", errors.New("what"))
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}
}
