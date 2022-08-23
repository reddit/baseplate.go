package prometheusbpint_test

import (
	"sync"
	"testing"

	"github.com/reddit/baseplate.go/internal/prometheusbpint"
)

func TestHighWatermarkValue(t *testing.T) {
	for _, c := range []struct {
		label string
		get   int64
		max   int64
		setup func(*prometheusbpint.HighWatermarkValue)
	}{
		{
			label: "inc-once",
			get:   1,
			max:   1,
			setup: func(hwv *prometheusbpint.HighWatermarkValue) {
				hwv.Inc()
			},
		},
		{
			label: "inc-inc-dec",
			get:   1,
			max:   2,
			setup: func(hwv *prometheusbpint.HighWatermarkValue) {
				hwv.Inc()
				hwv.Inc()
				hwv.Dec()
			},
		},
		{
			label: "dec-once",
			get:   -1,
			max:   0,
			setup: func(hwv *prometheusbpint.HighWatermarkValue) {
				hwv.Dec()
			},
		},
		{
			label: "set-2-3-5-8-1",
			get:   1,
			max:   8,
			setup: func(hwv *prometheusbpint.HighWatermarkValue) {
				hwv.Set(2)
				hwv.Set(3)
				hwv.Set(5)
				hwv.Set(8)
				hwv.Set(1)
			},
		},
		{
			label: "parallel-100",
			get:   100,
			max:   100,
			setup: func(hwv *prometheusbpint.HighWatermarkValue) {
				const n = 100
				var wg sync.WaitGroup
				wg.Add(n)
				for i := 0; i < n; i++ {
					go func() {
						hwv.Inc()
						wg.Done()
					}()
				}
				wg.Wait()
			},
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			var hwv prometheusbpint.HighWatermarkValue
			c.setup(&hwv)
			if got, want := hwv.Get(), c.get; got != want {
				t.Errorf("Get() got %d, want %d", got, want)
			}
			if got, want := hwv.Max(), c.max; got != want {
				t.Errorf("Max() got %d, want %d", got, want)
			}
		})
	}
}

func BenchmarkHighWatermarkValue(b *testing.B) {
	operations := [...]func(*prometheusbpint.HighWatermarkValue){
		func(hwv *prometheusbpint.HighWatermarkValue) {
			hwv.Inc()
		},
		func(hwv *prometheusbpint.HighWatermarkValue) {
			hwv.Dec()
		},
		func(hwv *prometheusbpint.HighWatermarkValue) {
			hwv.Set(2)
		},
		func(hwv *prometheusbpint.HighWatermarkValue) {
			hwv.Set(3)
		},
		func(hwv *prometheusbpint.HighWatermarkValue) {
			hwv.Set(5)
		},
		func(hwv *prometheusbpint.HighWatermarkValue) {
			hwv.Set(8)
		},
		func(hwv *prometheusbpint.HighWatermarkValue) {
			hwv.Get()
		},
		func(hwv *prometheusbpint.HighWatermarkValue) {
			hwv.Max()
		},
	}
	hwv := new(prometheusbpint.HighWatermarkValue)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var counter int
		for pb.Next() {
			counter++
			n := counter % len(operations)
			operations[n](hwv)
		}
	})
}
