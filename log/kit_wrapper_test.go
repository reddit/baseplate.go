package log_test

import (
	"testing"

	kitlog "github.com/go-kit/kit/log"

	"github.com/reddit/baseplate.go/log"
)

func TestKitWrapper(t *testing.T) {
	// This is cheating :)
	// We don't do any tests here, just make sure that log.KitLogger is indeed
	// an implementation of kitlog.Logger interface.
	//
	// Usually that's done in the main code with a variable declaration,
	// but in this case that will pull kitlog into the dependency tree of this
	// package, which we want to avoid.
	//
	// So do it in test instead.
	// If in the future any interface changes breaks it,
	// that would also break this test.
	var _ kitlog.Logger = log.KitLogger(log.ErrorLevel)
}
