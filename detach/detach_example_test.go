package detach_test

import (
	"context"
	"fmt"
	"time"

	"github.com/reddit/baseplate.go/detach"
)

func init() {
	detach.Register(detach.Hooks{
		Inline: func(dst, src context.Context) context.Context {
			if v, ok := src.Value(contextKey).(*ctxVal); ok && v != nil {
				dst = context.WithValue(dst, contextKey, v)
			}
			return dst
		},
		Async: func(dst, src context.Context, next func(ctx context.Context)) {
			if v, ok := src.Value(contextKey).(*ctxVal); ok && v != nil {
				newVal := &ctxVal{val: v.val}
				dst = context.WithValue(dst, contextKey, newVal)
				defer func() {
					fmt.Printf("newVal.closed() == %v\n", newVal.closed)
					newVal.close()
					fmt.Printf("newVal.closed() == %v\n", newVal.closed)
				}()
			}
			next(dst)
		},
	})
}

type contextKeyType struct{}

var contextKey contextKeyType

type ctxVal struct {
	val    int
	closed bool
}

func (v *ctxVal) close() {
	v.closed = true
}

func ExampleInline() {
	const parentTimeout = time.Millisecond

	v := &ctxVal{val: 1}

	ctx, cancel := context.WithTimeout(context.Background(), parentTimeout)
	defer cancel()

	ctx = context.WithValue(ctx, contextKey, v)

	defer func() {
		detachedCtx, detachedCancel := detach.Inline(ctx, 100*parentTimeout)
		defer detachedCancel()

		fmt.Printf("parent.Err() == %v\n", ctx.Err())
		fmt.Printf("detached.Err() ==  %v\n", detachedCtx.Err())
		detachedV, _ := detachedCtx.Value(contextKey).(*ctxVal)
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
		time.Sleep(5 * parentTimeout)
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

func ExampleAsync() {
	const parentTimeout = time.Millisecond

	// asyncDone is used in this example to let us wait until the async call finishes so we can test the output, in real
	// code you probably wouldn't do this.
	asyncDone := make(chan bool, 1)

	run := func() {
		v := &ctxVal{val: 1}
		defer v.close()

		ctx, cancel := context.WithTimeout(context.Background(), parentTimeout)
		defer cancel()

		ctx = context.WithValue(ctx, contextKey, v)

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

		go detach.Async(ctx, 100*parentTimeout, func(detachedCtx context.Context) {
			fmt.Printf("parent.Err() == %v\n", ctx.Err())
			fmt.Printf("detached.Err() ==  %v\n", detachedCtx.Err())
			detachedV, _ := detachedCtx.Value(contextKey).(*ctxVal)
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
				fmt.Println("finished cleanup")
			}

			asyncDone <- true
		})
	}

	fmt.Println("running...")
	run()

	fmt.Println("waiting for async cleanup")
	<-asyncDone

	// Output:
	// running...
	// timed out
	// waiting for async cleanup
	// parent.Err() == context deadline exceeded
	// detached.Err() ==  <nil>
	// value equal: false
	// value.val equal: true
	// finished cleanup
	// newVal.closed() == false
	// newVal.closed() == true
}
