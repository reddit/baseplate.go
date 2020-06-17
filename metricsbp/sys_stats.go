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
	const prefix = "runtime."

	st = st.fallback()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "UNKOWN-HOSTNAME"
	}
	tags := Tags{
		"instance": hostname,
		"pid":      fmt.Sprintf("PID%d", os.Getpid()),
	}.AsStatsdTags()

	// init the gauges
	// cpu
	cpuGoroutines := st.Gauge(prefix + "cpu.goroutines").With(tags...)
	cpuCgoCalls := st.Gauge(prefix + "cpu.cgo_calls").With(tags...)
	// gc
	gcSys := st.Gauge(prefix + "mem.gc.sys").With(tags...)
	gcNext := st.Gauge(prefix + "mem.gc.next").With(tags...)
	gcLast := st.Gauge(prefix + "mem.gc.last").With(tags...)
	gcPauseTotal := st.Gauge(prefix + "mem.gc.pause_total").With(tags...)
	gcPause := st.Gauge(prefix + "mem.gc.pause").With(tags...)
	gcCount := st.Gauge(prefix + "mem.gc.count").With(tags...)
	// general
	memAlloc := st.Gauge(prefix + "mem.alloc").With(tags...)
	memTotal := st.Gauge(prefix + "mem.total").With(tags...)
	memSys := st.Gauge(prefix + "mem.sys").With(tags...)
	memLookups := st.Gauge(prefix + "mem.lookups").With(tags...)
	memMalloc := st.Gauge(prefix + "mem.malloc").With(tags...)
	memFrees := st.Gauge(prefix + "mem.frees").With(tags...)
	// heap
	heapAlloc := st.Gauge(prefix + "mem.heap.alloc").With(tags...)
	heapSys := st.Gauge(prefix + "mem.heap.sys").With(tags...)
	heapIdle := st.Gauge(prefix + "mem.heap.idle").With(tags...)
	heapInuse := st.Gauge(prefix + "mem.heap.inuse").With(tags...)
	heapReleased := st.Gauge(prefix + "mem.heap.released").With(tags...)
	heapObjects := st.Gauge(prefix + "mem.heap.objects").With(tags...)
	// stack
	stackInuse := st.Gauge(prefix + "mem.stack.inuse").With(tags...)
	stackSys := st.Gauge(prefix + "mem.stack.sys").With(tags...)
	mspanInuse := st.Gauge(prefix + "mem.stack.mspan_inuse").With(tags...)
	mspanSys := st.Gauge(prefix + "mem.stack.mspan_sys").With(tags...)
	mcacheInuse := st.Gauge(prefix + "mem.stack.mcache_inuse").With(tags...)
	mcacheSys := st.Gauge(prefix + "mem.stack.mcache_sys").With(tags...)
	// other
	memOther := st.Gauge(prefix + "mem.othersys").With(tags...)
	activeRequests := st.Gauge(prefix + "active_requests").With(tags...)

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
				activeRequests.Set(float64(st.getActiveRequests()))
			}
		}
	}()
}
