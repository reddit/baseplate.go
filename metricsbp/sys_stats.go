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
func (st Statsd) RunSysStats() {
	// init the gauges
	// cpu
	cpuGoroutines := st.Gauge("cpu.goroutines")
	cpuCgoCalls := st.Gauge("cpu.cgo_calls")
	// gc
	gcSys := st.Gauge("mem.gc.sys")
	gcNext := st.Gauge("mem.gc.next")
	gcLast := st.Gauge("mem.gc.last")
	gcPauseTotal := st.Gauge("mem.gc.pause_total")
	gcPause := st.Gauge("mem.gc.pause")
	gcCount := st.Gauge("mem.gc.count")
	// general
	memAlloc := st.Gauge("mem.alloc")
	memTotal := st.Gauge("mem.total")
	memSys := st.Gauge("mem.sys")
	memLookups := st.Gauge("mem.lookups")
	memMalloc := st.Gauge("mem.malloc")
	memFrees := st.Gauge("mem.frees")
	// heap
	heapAlloc := st.Gauge("mem.heap.alloc")
	heapSys := st.Gauge("mem.heap.sys")
	heapIdle := st.Gauge("mem.heap.idle")
	heapInuse := st.Gauge("mem.heap.inuse")
	heapReleased := st.Gauge("mem.heap.released")
	heapObjects := st.Gauge("mem.heap.objects")
	// stack
	stackInuse := st.Gauge("mem.stack.inuse")
	stackSys := st.Gauge("mem.stack.sys")
	mspanInuse := st.Gauge("mem.stack.mspan_inuse")
	mspanSys := st.Gauge("mem.stack.mspan_sys")
	mcacheInuse := st.Gauge("mem.stack.mcache_inuse")
	mcacheSys := st.Gauge("mem.stack.mcache_sys")
	// other
	memOther := st.Gauge("mem.othersys")

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
