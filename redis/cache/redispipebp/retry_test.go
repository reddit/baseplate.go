package redispipebp_test

import (
	"context"
	"testing"

	"github.com/avast/retry-go"
	"github.com/joomcode/redispipe/redis"

	"github.com/reddit/baseplate.go/retrybp"

	"github.com/reddit/baseplate.go/redis/cache/redispipebp"
	"github.com/reddit/baseplate.go/redis/cache/redisx"
)

type counts struct {
	do              uint
	send            uint
	sendMany        uint
	sendTransaction uint
	scannerNext     uint
}

type counterSync struct {
	s redisx.Sync

	count *counts
}

func (c counterSync) Do(ctx context.Context, cmd string, args ...interface{}) interface{} {
	c.count.do++
	return c.s.Do(ctx, cmd, args...)
}

func (c counterSync) Send(ctx context.Context, req redis.Request) interface{} {
	c.count.send++
	return c.s.Send(ctx, req)
}

func (c counterSync) SendMany(ctx context.Context, reqs []redis.Request) []interface{} {
	c.count.sendMany++
	return c.s.SendMany(ctx, reqs)
}

func (c counterSync) SendTransaction(ctx context.Context, reqs []redis.Request) ([]interface{}, error) {
	c.count.sendTransaction++
	return c.s.SendTransaction(ctx, reqs)
}

func (c counterSync) Scanner(ctx context.Context, opts redis.ScanOpts) redisx.ScanIterator {
	return counterScanner{
		s:     c.s.Scanner(ctx, opts),
		count: c.count,
	}
}

type counterScanner struct {
	s     redisx.ScanIterator
	count *counts
}

func (c counterScanner) Next() ([]string, error) {
	c.count.scannerNext++
	return c.s.Next()
}

func TestRetrySync(t *testing.T) {
	defer flushRedis()

	const (
		attempts uint = 3

		do              = "Do"
		send            = "Send"
		sendMany        = "SendMany"
		sendTransaction = "SendTransaction"
		scanner         = "Scanner"
	)
	ctx := context.Background()
	cs := counterSync{
		s:     alwaysError{},
		count: &counts{},
	}
	rClient := redispipebp.RetrySync{
		Sync: cs,
		DefaultOpts: []retry.Option{
			retry.Attempts(attempts),
			retrybp.Filters(func(err error, next retry.RetryIfFunc) bool {
				return true
			}),
		},
	}

	t.Run(do, func(t *testing.T) {
		if cs.count.do != 0 {
			t.Fatalf("wrong count for %q, expected %d, got %d", do, 0, cs.count.do)
		}
		_ = rClient.Do(ctx, "PING")
		if cs.count.do != attempts {
			t.Fatalf("wrong count for %q, expected %d, got %d", do, attempts, cs.count.do)
		}
	})

	t.Run(send, func(t *testing.T) {
		if cs.count.send != 0 {
			t.Fatalf("wrong count for %q, expected %d, got %d", send, 0, cs.count.send)
		}
		_ = rClient.Send(ctx, redis.Req("PING"))
		if cs.count.send != attempts {
			t.Fatalf("wrong count for %q, expected %d, got %d", send, attempts, cs.count.send)
		}
	})

	t.Run(sendMany, func(t *testing.T) {
		if cs.count.sendMany != 0 {
			t.Fatalf("wrong count for %q, expected %d, got %d", sendMany, 0, cs.count.sendMany)
		}
		_ = rClient.SendMany(ctx, []redis.Request{
			redis.Req("PING"),
			redis.Req("SET", "key", "value"),
		})
		if cs.count.sendMany != attempts {
			t.Fatalf("wrong count for %q, expected %d, got %d", sendMany, attempts, cs.count.sendMany)
		}
	})

	t.Run(sendTransaction, func(t *testing.T) {
		if cs.count.sendTransaction != 0 {
			t.Fatalf("wrong count for %q, expected %d, got %d", sendTransaction, 0, cs.count.sendTransaction)
		}
		_, _ = rClient.SendTransaction(ctx, []redis.Request{
			redis.Req("PING"),
			redis.Req("SET", "key", "value"),
		})
		if cs.count.sendTransaction != attempts {
			t.Fatalf("wrong count for %q, expected %d, got %d", sendTransaction, attempts, cs.count.sendTransaction)
		}
	})

	t.Run(scanner, func(t *testing.T) {
		if cs.count.scannerNext != 0 {
			t.Fatalf("wrong count for %q, expected %d, got %d", scanner, 0, cs.count.scannerNext)
		}
		s := rClient.Scanner(ctx, redis.ScanOpts{
			Cmd:   "KEYS",
			Match: "*",
		})
		_, _ = s.Next()
		if cs.count.scannerNext != attempts {
			t.Fatalf("wrong count for %q, expected %d, got %d", scanner, attempts, cs.count.scannerNext)
		}
	})
}
