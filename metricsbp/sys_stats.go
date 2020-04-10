package metricsbp

import (
	"runtime"
	"time"
)

// SysStatsTickerInterval is the interval we pull and report sys stats.
// Default is 10 seconds.
var SysStatsTickerInterval = time.Second * 10

type cpuStats struct {
	NumGoroutine int
	NumCgoCall   int64
}

func pullRuntimeStats() (cpu cpuStats, mem runtime.MemStats) {
	runtime.ReadMemStats(&mem)
	cpu.NumGoroutine = runtime.NumGoroutine()
	cpu.NumCgoCall = runtime.NumCgoCall()
	return
}

// RunSysStats starts a goroutine to periodically pull and report sys stats.
//
// Canceling the context passed into NewStatsd will stop this goroutine.
func (st *Statsd) RunSysStats(labels Labels) {
	st = st.fallback()

	l := labels.AsStatsdLabels()

	// init the gauges
	// cpu
	cpuGoroutines := st.Gauge("cpu.goroutines").With(l...)
	cpuCgoCalls := st.Gauge("cpu.cgo_calls").With(l...)
	// gc
	gcSys := st.Gauge("mem.gc.sys").With(l...)
	gcNext := st.Gauge("mem.gc.next").With(l...)
	gcLast := st.Gauge("mem.gc.last").With(l...)
	gcPauseTotal := st.Gauge("mem.gc.pause_total").With(l...)
	gcPause := st.Gauge("mem.gc.pause").With(l...)
	gcCount := st.Gauge("mem.gc.count").With(l...)
	// general
	memAlloc := st.Gauge("mem.alloc").With(l...)
	memTotal := st.Gauge("mem.total").With(l...)
	memSys := st.Gauge("mem.sys").With(l...)
	memLookups := st.Gauge("mem.lookups").With(l...)
	memMalloc := st.Gauge("mem.malloc").With(l...)
	memFrees := st.Gauge("mem.frees").With(l...)
	// heap
	heapAlloc := st.Gauge("mem.heap.alloc").With(l...)
	heapSys := st.Gauge("mem.heap.sys").With(l...)
	heapIdle := st.Gauge("mem.heap.idle").With(l...)
	heapInuse := st.Gauge("mem.heap.inuse").With(l...)
	heapReleased := st.Gauge("mem.heap.released").With(l...)
	heapObjects := st.Gauge("mem.heap.objects").With(l...)
	// stack
	stackInuse := st.Gauge("mem.stack.inuse").With(l...)
	stackSys := st.Gauge("mem.stack.sys").With(l...)
	mspanInuse := st.Gauge("mem.stack.mspan_inuse").With(l...)
	mspanSys := st.Gauge("mem.stack.mspan_sys").With(l...)
	mcacheInuse := st.Gauge("mem.stack.mcache_inuse").With(l...)
	mcacheSys := st.Gauge("mem.stack.mcache_sys").With(l...)
	// other
	memOther := st.Gauge("mem.othersys").With(l...)

	go func() {
		ticker := time.NewTicker(SysStatsTickerInterval)
		defer ticker.Stop()

		for {
			select {
			case <-st.ctx.Done():
				return
			case <-ticker.C:
				cpu, mem := pullRuntimeStats()

				// cpu
				cpuGoroutines.Set(float64(cpu.NumGoroutine))
				cpuCgoCalls.Set(float64(cpu.NumCgoCall))
				// gc
				gcSys.Set(float64(mem.GCSys))
				gcNext.Set(float64(mem.NextGC))
				gcLast.Set(float64(mem.LastGC))
				gcPauseTotal.Set(float64(mem.PauseTotalNs))
				gcPause.Set(float64(mem.PauseNs[(mem.NumGC+255)%256]))
				gcCount.Set(float64(mem.NumGC))
				// general
				memAlloc.Set(float64(mem.Alloc))
				memTotal.Set(float64(mem.TotalAlloc))
				memSys.Set(float64(mem.Sys))
				memLookups.Set(float64(mem.Lookups))
				memMalloc.Set(float64(mem.Mallocs))
				memFrees.Set(float64(mem.Frees))
				// heap
				heapAlloc.Set(float64(mem.HeapAlloc))
				heapSys.Set(float64(mem.HeapSys))
				heapIdle.Set(float64(mem.HeapIdle))
				heapInuse.Set(float64(mem.HeapInuse))
				heapReleased.Set(float64(mem.HeapReleased))
				heapObjects.Set(float64(mem.HeapObjects))
				// stack
				stackInuse.Set(float64(mem.StackInuse))
				stackSys.Set(float64(mem.StackSys))
				mspanInuse.Set(float64(mem.MSpanInuse))
				mspanSys.Set(float64(mem.MSpanSys))
				mcacheInuse.Set(float64(mem.MCacheInuse))
				mcacheSys.Set(float64(mem.MCacheSys))
				// other
				memOther.Set(float64(mem.OtherSys))
			}
		}
	}()
}
