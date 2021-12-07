package detach_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/reddit/baseplate.go/detach"
)

// In a real package, you should register hooks for both inline and async
// calls at the same time.
//
// It should likely be done within an `init()` function rather than calling the
// function explicitly.
func InitAsync() {
	detach.Register(detach.Hooks{
		Inline: nil,
		Async: func(dst, src context.Context, next func(ctx context.Context)) {
			if v, ok := src.Value(asyncContextKey).(*asyncContextVal); ok && v != nil {
				newVal := &asyncContextVal{val: v.val}
				dst = context.WithValue(dst, asyncContextKey, newVal)
				defer func() {
					newVal.close()
				}()
			}
			next(dst)
		},
	})
}

type asyncContextKeyType struct{}

var asyncContextKey asyncContextKeyType

type asyncContextVal struct {
	mu sync.Mutex

	val    int
	closed bool
}

func (v *asyncContextVal) close() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.closed = true
}

func ExampleAsync() {
	InitAsync()

	const parentTimeout = time.Millisecond

	// asyncDone is used in this example to let us wait until the async call
	// finishes, so we can test the output. In real code you probably wouldn't do
	// this.
	asyncDone := sync.WaitGroup{}
	asyncDone.Add(1)

	run := func() {
		v := &asyncContextVal{val: 1}
		defer v.close()

		ctx, cancel := context.WithTimeout(context.Background(), parentTimeout)
		defer cancel()

		ctx = context.WithValue(ctx, asyncContextKey, v)

		ch := make(chan bool, 1)
		go func() {
			time.Sleep(5 * parentTimeout)
			ch <- true
		}()

		select {
		case <-ch:
			return
		case <-ctx.Done():
		}
		fmt.Println("timed out")

		go detach.Async(ctx, func(detachedCtx context.Context) {
			detachedCtx, detachedCancel := context.WithTimeout(detachedCtx, 100*parentTimeout)
			defer detachedCancel()

			fmt.Printf("parent.Err() == %v\n", ctx.Err())
			fmt.Printf("detached.Err() ==  %v\n", detachedCtx.Err())
			detachedV, _ := detachedCtx.Value(asyncContextKey).(*asyncContextVal)
			fmt.Printf("value equal: %v\n", v == detachedV)
			fmt.Printf("value.val equal: %v\n", v.val == detachedV.val)

			ch := make(chan bool, 1)
			go func() {
				time.Sleep(parentTimeout)
				ch <- true
			}()

			select {
			case <-detachedCtx.Done():
				fmt.Println("never happens")
			case <-ch:
			}

			asyncDone.Done()
		})
	}

	fmt.Println("running...")
	run()

	fmt.Println("waiting for async cleanup")
	asyncDone.Wait()
	fmt.Println("finished cleanup")

	// Output:
	// running...
	// timed out
	// waiting for async cleanup
	// parent.Err() == context deadline exceeded
	// detached.Err() ==  <nil>
	// value equal: false
	// value.val equal: true
	// finished cleanup
}
