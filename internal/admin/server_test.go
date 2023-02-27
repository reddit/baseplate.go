package admin

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var expectedMetrics = []string{
	"go_gc_duration_seconds",
	"go_goroutines",
	"go_info",
	"go_memstats_alloc_bytes",
	"go_memstats_alloc_bytes_total",
	"go_memstats_buck_hash_sys_bytes",
	"go_memstats_frees_total",
	"go_memstats_gc_sys_bytes",
	"go_memstats_heap_alloc_bytes",
	"go_memstats_heap_idle_bytes",
	"go_memstats_heap_inuse_bytes",
	"go_memstats_heap_objects",
	"go_memstats_heap_released_bytes",
	"go_memstats_heap_sys_bytes",
	"go_memstats_last_gc_time_seconds",
	"go_memstats_lookups_total",
	"go_memstats_mallocs_total",
	"go_memstats_mcache_inuse_bytes",
	"go_memstats_mcache_sys_bytes",
	"go_memstats_mspan_inuse_bytes",
	"go_memstats_mspan_sys_bytes",
	"go_memstats_next_gc_bytes",
	"go_memstats_other_sys_bytes",
	"go_memstats_stack_inuse_bytes",
	"go_memstats_stack_sys_bytes",
	"go_memstats_sys_bytes",
	"go_sched_gomaxprocs_threads",
	"go_sched_goroutines_goroutines",
	"go_sched_latencies_seconds",
	"go_threads",
}

func TestMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(baseplateGoCollectors))

	result, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	got := []string{}
	for _, r := range result {
		got = append(got, r.GetName())
	}

	if diff := cmp.Diff(expectedMetrics, got); diff != "" {
		t.Errorf("registered metrics mismatch (-want +got):\n%s", diff)
	}
}
