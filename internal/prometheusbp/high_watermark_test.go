package prometheusbp_test

import (
	"sync"
	"testing"

	"github.com/reddit/baseplate.go/internal/prometheusbp"
)

func TestHighWatermarkValue(t *testing.T) {
	for _, c := range []struct {
		label string
		get   int64
		max   int64
		setup func(*prometheusbp.HighWatermarkValue)
	}{
		{
			label: "inc-once",
			get:   1,
			max:   1,
			setup: func(hwv *prometheusbp.HighWatermarkValue) {
				hwv.Inc()
			},
		},
		{
			label: "inc-inc-dec",
			get:   1,
			max:   2,
			setup: func(hwv *prometheusbp.HighWatermarkValue) {
				hwv.Inc()
				hwv.Inc()
				hwv.Dec()
			},
		},
		{
			label: "dec-once",
			get:   -1,
			max:   0,
			setup: func(hwv *prometheusbp.HighWatermarkValue) {
				hwv.Dec()
			},
		},
		{
			label: "set-2-3-5-8-1",
			get:   1,
			max:   8,
			setup: func(hwv *prometheusbp.HighWatermarkValue) {
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
			setup: func(hwv *prometheusbp.HighWatermarkValue) {
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
			var hwv prometheusbp.HighWatermarkValue
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
	operations := [...]func(*prometheusbp.HighWatermarkValue){
		func(hwv *prometheusbp.HighWatermarkValue) {
			hwv.Inc()
		},
		func(hwv *prometheusbp.HighWatermarkValue) {
			hwv.Dec()
		},
		func(hwv *prometheusbp.HighWatermarkValue) {
			hwv.Set(2)
		},
		func(hwv *prometheusbp.HighWatermarkValue) {
			hwv.Set(3)
		},
		func(hwv *prometheusbp.HighWatermarkValue) {
			hwv.Set(5)
		},
		func(hwv *prometheusbp.HighWatermarkValue) {
			hwv.Set(8)
		},
		func(hwv *prometheusbp.HighWatermarkValue) {
			hwv.Get()
		},
		func(hwv *prometheusbp.HighWatermarkValue) {
			hwv.Max()
		},
	}
	hwv := new(prometheusbp.HighWatermarkValue)

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
