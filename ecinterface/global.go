package ecinterface

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
	"github.com/reddit/baseplate.go/log"
)

const (
	promNamespace = "ecinterface"
)

var (
	getBeforeSet = promauto.With(prometheusbpint.GlobalRegistry).NewCounter(prometheus.CounterOpts{
		Namespace: promNamespace,
		Name:      "get_before_set_total",
		Help:      "Total number of ecinterface.Get calls before Set is called",
	})
)

// Logger is used by Get when it's called before Set is called.
var Logger log.Wrapper

// ErrGetBeforeSet is the error returned when Get is called before Set.
var ErrGetBeforeSet = errors.New("ecinterface: Get called before Set is called")

// current is the storage type of global.
//
// atomic.Value requires that the underlying concrete type remain constant.
// If we try to store two different implementations of Interface, we will get a panic,
// because Interface is promoted to any when you call Store.
//
// Thus, we use a `current{}` so that the concrete type is always the same.
type current struct {
	Interface
}

// actual type: current
var global atomic.Value

// Set sets the global edge context implementation.
func Set(impl Interface) {
	global.Store(current{impl})
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
	stored := global.Load()
	if stored == nil {
		Logger.Log(context.Background(), ErrGetBeforeSet.Error())
		getBeforeSet.Inc()
		return nopImpl
	}
	return stored.(current).Interface
}

type nop struct{}

var nopImpl nop

func (nop) HeaderToContext(ctx context.Context, header string) (context.Context, error) {
	return ctx, ErrGetBeforeSet
}

func (nop) ContextToHeader(ctx context.Context) (string, bool) {
	return "", false
}
