package metricsbp

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/go-kit/kit/metrics"
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
// All the sys stats will be reported as RuntimeGauges.
//
// Canceling the context passed into NewStatsd will stop this goroutine.
func (st *Statsd) RunSysStats() {
	st = st.fallback()

	// init the gauges
	// cpu
	cpuGoroutines := st.RuntimeGauge("cpu.goroutines")
	cpuCgoCalls := st.RuntimeGauge("cpu.cgo_calls")
	// gc
	gcSys := st.RuntimeGauge("mem.gc.sys")
	gcNext := st.RuntimeGauge("mem.gc.next")
	gcLast := st.RuntimeGauge("mem.gc.last")
	gcPauseTotal := st.RuntimeGauge("mem.gc.pause_total")
	gcPause := st.RuntimeGauge("mem.gc.pause")
	gcCount := st.RuntimeGauge("mem.gc.count")
	// general
	memAlloc := st.RuntimeGauge("mem.alloc")
	memTotal := st.RuntimeGauge("mem.total")
	memSys := st.RuntimeGauge("mem.sys")
	memLookups := st.RuntimeGauge("mem.lookups")
	memMalloc := st.RuntimeGauge("mem.malloc")
	memFrees := st.RuntimeGauge("mem.frees")
	// heap
	heapAlloc := st.RuntimeGauge("mem.heap.alloc")
	heapSys := st.RuntimeGauge("mem.heap.sys")
	heapIdle := st.RuntimeGauge("mem.heap.idle")
	heapInuse := st.RuntimeGauge("mem.heap.inuse")
	heapReleased := st.RuntimeGauge("mem.heap.released")
	heapObjects := st.RuntimeGauge("mem.heap.objects")
	// stack
	stackInuse := st.RuntimeGauge("mem.stack.inuse")
	stackSys := st.RuntimeGauge("mem.stack.sys")
	mspanInuse := st.RuntimeGauge("mem.stack.mspan_inuse")
	mspanSys := st.RuntimeGauge("mem.stack.mspan_sys")
	mcacheInuse := st.RuntimeGauge("mem.stack.mcache_inuse")
	mcacheSys := st.RuntimeGauge("mem.stack.mcache_sys")
	// other
	memOther := st.RuntimeGauge("mem.othersys")
	activeRequests := st.RuntimeGauge("active_requests")

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

const runtimeGaugePrefix = "runtime."

// runtimeGaugeTags will be initialized by runtimeGaugeTagsOnce,
// in getRuntimeGaugeTags.
var (
	runtimeGaugeTags     []string
	runtimeGaugeTagsOnce sync.Once
)

func getRuntimeGaugeTags() []string {
	runtimeGaugeTagsOnce.Do(func() {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "UNKNOWN-HOSTNAME"
		}
		runtimeGaugeTags = Tags{
			"instance": hostname,
			"pid":      fmt.Sprintf("PID%d", os.Getpid()),
		}.AsStatsdTags()
	})
	return runtimeGaugeTags
}

// RuntimeGauge returns a Gauge that's suitable to report runtime data.
//
// It will be applied with "runtime." prefix and instance+pid tags
// automatically.
//
// All gauges reported from RunSysStats are runtime gauges.
func (st *Statsd) RuntimeGauge(name string) metrics.Gauge {
	return st.Gauge(runtimeGaugePrefix + name).With(getRuntimeGaugeTags()...)
}
