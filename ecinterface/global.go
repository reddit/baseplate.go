package ecinterface

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
	"github.com/reddit/baseplate.go/log"
)

const (
	promNamespace = "ecinterface"
)

var (
	getBeforeSet = promauto.With(internalv2compat.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Namespace: promNamespace,
		Name:      "get_before_set_total",
		Help:      "Total number of ecinterface.Get calls before Set is called",
	})
)

// Logger is used by Get when it's called before Set is called.
var Logger log.Wrapper

// ErrGetBeforeSet is the error returned when Get is called before Set.
var ErrGetBeforeSet = errors.New("ecinterface: Get called before Set is called")

// actual type: Interface
var global atomic.Value

// Set sets the global edge context implementation.
func Set(impl Interface) {
	global.Store(impl)
}

// Get returns the previously Set global edge context implementation.
//
// It's guaranteed to return a non-nil implementation.
// If it's called before any Set is called,
// it logs the event (via Logger),
// then returns an implementation that does nothing:
//
// - Its HeaderToContext always return the context intact with ErrGetBeforeSet.
//
// - Its ContextToHeader always return ("", false).
func Get() Interface {
	v := global.Load()
	if impl, _ := v.(Interface); impl != nil {
		return impl
	}
	Logger.Log(context.Background(), ErrGetBeforeSet.Error())
	getBeforeSet.Inc()
	return nopImpl
}

type nop struct{}

var nopImpl nop

func (nop) HeaderToContext(ctx context.Context, header string) (context.Context, error) {
	return ctx, ErrGetBeforeSet
}

func (nop) ContextToHeader(ctx context.Context) (string, bool) {
	return "", false
}
