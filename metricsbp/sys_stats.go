package metricsbp

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// SysStatsTickerInterval is the interval we pull and report sys stats.
// Default is 10 seconds.
var SysStatsTickerInterval = time.Second * 10

var activeRequestCounter int64

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
func (st *Statsd) RunSysStats() {
	st = st.fallback()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "UNKOWN-HOSTNAME"
	}
	prefix := fmt.Sprintf("runtime.%s.PID%d.", hostname, os.Getpid())

	// init the gauges
	// cpu
	cpuGoroutines := st.Gauge(prefix + "cpu.goroutines")
	cpuCgoCalls := st.Gauge(prefix + "cpu.cgo_calls")
	// gc
	gcSys := st.Gauge(prefix + "mem.gc.sys")
	gcNext := st.Gauge(prefix + "mem.gc.next")
	gcLast := st.Gauge(prefix + "mem.gc.last")
	gcPauseTotal := st.Gauge(prefix + "mem.gc.pause_total")
	gcPause := st.Gauge(prefix + "mem.gc.pause")
	gcCount := st.Gauge(prefix + "mem.gc.count")
	// general
	memAlloc := st.Gauge(prefix + "mem.alloc")
	memTotal := st.Gauge(prefix + "mem.total")
	memSys := st.Gauge(prefix + "mem.sys")
	memLookups := st.Gauge(prefix + "mem.lookups")
	memMalloc := st.Gauge(prefix + "mem.malloc")
	memFrees := st.Gauge(prefix + "mem.frees")
	// heap
	heapAlloc := st.Gauge(prefix + "mem.heap.alloc")
	heapSys := st.Gauge(prefix + "mem.heap.sys")
	heapIdle := st.Gauge(prefix + "mem.heap.idle")
	heapInuse := st.Gauge(prefix + "mem.heap.inuse")
	heapReleased := st.Gauge(prefix + "mem.heap.released")
	heapObjects := st.Gauge(prefix + "mem.heap.objects")
	// stack
	stackInuse := st.Gauge(prefix + "mem.stack.inuse")
	stackSys := st.Gauge(prefix + "mem.stack.sys")
	mspanInuse := st.Gauge(prefix + "mem.stack.mspan_inuse")
	mspanSys := st.Gauge(prefix + "mem.stack.mspan_sys")
	mcacheInuse := st.Gauge(prefix + "mem.stack.mcache_inuse")
	mcacheSys := st.Gauge(prefix + "mem.stack.mcache_sys")
	// other
	memOther := st.Gauge(prefix + "mem.othersys")

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
