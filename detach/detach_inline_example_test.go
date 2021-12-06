package detach_test

import (
	"context"
	"fmt"
	"time"

	"github.com/reddit/baseplate.go/detach"
)

// In a real package, you should register hooks for both inline and async
// calls at the same time.
//
// It should likely be done within an `init()` function rather than calling the
// function explicitly.
func InitInline() {
	detach.Register(detach.Hooks{
		Inline: func(dst, src context.Context) context.Context {
			if v, ok := src.Value(inlineContextKey).(*inlineContextVal); ok && v != nil {
				dst = context.WithValue(dst, inlineContextKey, v)
			}
			return dst
		},
		Async: nil,
	})
}

type inlineContextKeyType struct{}

var inlineContextKey inlineContextKeyType

type inlineContextVal struct {
	val    int
	closed bool
}

func (v *inlineContextVal) close() {
	v.closed = true
}

func ExampleInline() {
	InitInline()

	const parentTimeout = time.Millisecond

	v := &inlineContextVal{val: 1}
	defer v.close()

	ctx, cancel := context.WithTimeout(context.Background(), parentTimeout)
	defer cancel()

	ctx = context.WithValue(ctx, inlineContextKey, v)

	defer func() {
		detachedCtx, detachedCancel := detach.Inline(ctx, 100*parentTimeout)
		defer detachedCancel()

		fmt.Printf("parent.Err() == %v\n", ctx.Err())
		fmt.Printf("detached.Err() ==  %v\n", detachedCtx.Err())
		detachedV, _ := detachedCtx.Value(inlineContextKey).(*inlineContextVal)
		fmt.Printf("value equal: %v\n", v == detachedV)
		fmt.Printf("value closed: %v\n", detachedV.closed)

		ch := make(chan bool, 1)
		go func() {
			time.Sleep(parentTimeout)
			ch <- true
		}()

		select {
		case <-detachedCtx.Done():
			fmt.Println("never happens")
		case <-ch:
			fmt.Println("finished cleanup")
		}
	}()

	ch := make(chan bool, 1)
	go func() {
		time.Sleep(100 * parentTimeout)
		ch <- true
	}()

	select {
	case <-ctx.Done():
		return
	case <-ch:
		fmt.Println("never happens")
	}

	// Output:
	// parent.Err() == context deadline exceeded
	// detached.Err() ==  <nil>
	// value equal: true
	// value closed: false
	// finished cleanup
}
