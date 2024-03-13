package admin

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var expectedMetrics = []string{
	"go_cgo_go_to_c_calls_calls_total",
	"go_cpu_classes_gc_mark_assist_cpu_seconds_total",
	"go_cpu_classes_gc_mark_dedicated_cpu_seconds_total",
	"go_cpu_classes_gc_mark_idle_cpu_seconds_total",
	"go_cpu_classes_gc_pause_cpu_seconds_total",
	"go_cpu_classes_gc_total_cpu_seconds_total",
	"go_cpu_classes_idle_cpu_seconds_total",
	"go_cpu_classes_scavenge_assist_cpu_seconds_total",
	"go_cpu_classes_scavenge_background_cpu_seconds_total",
	"go_cpu_classes_scavenge_total_cpu_seconds_total",
	"go_cpu_classes_total_cpu_seconds_total",
	"go_cpu_classes_user_cpu_seconds_total",
	"go_gc_cycles_automatic_gc_cycles_total",
	"go_gc_cycles_forced_gc_cycles_total",
	"go_gc_cycles_total_gc_cycles_total",
	"go_gc_duration_seconds",
	"go_gc_gogc_percent",
	"go_gc_gomemlimit_bytes",
	"go_gc_heap_allocs_by_size_bytes",
	"go_gc_heap_allocs_bytes_total",
	"go_gc_heap_allocs_objects_total",
	"go_gc_heap_frees_by_size_bytes",
	"go_gc_heap_frees_bytes_total",
	"go_gc_heap_frees_objects_total",
	"go_gc_heap_goal_bytes",
	"go_gc_heap_live_bytes",
	"go_gc_heap_objects_objects",
	"go_gc_heap_tiny_allocs_objects_total",
	"go_gc_limiter_last_enabled_gc_cycle",
	"go_gc_pauses_seconds",
	"go_gc_scan_globals_bytes",
	"go_gc_scan_heap_bytes",
	"go_gc_scan_stack_bytes",
	"go_gc_scan_total_bytes",
	"go_gc_stack_starting_size_bytes",
	"go_godebug_non_default_behavior_execerrdot_events_total",
	"go_godebug_non_default_behavior_gocachehash_events_total",
	"go_godebug_non_default_behavior_gocachetest_events_total",
	"go_godebug_non_default_behavior_gocacheverify_events_total",
	"go_godebug_non_default_behavior_http2client_events_total",
	"go_godebug_non_default_behavior_http2server_events_total",
	"go_godebug_non_default_behavior_installgoroot_events_total",
	"go_godebug_non_default_behavior_jstmpllitinterp_events_total",
	"go_godebug_non_default_behavior_multipartmaxheaders_events_total",
	"go_godebug_non_default_behavior_multipartmaxparts_events_total",
	"go_godebug_non_default_behavior_multipathtcp_events_total",
	"go_godebug_non_default_behavior_panicnil_events_total",
	"go_godebug_non_default_behavior_randautoseed_events_total",
	"go_godebug_non_default_behavior_tarinsecurepath_events_total",
	"go_godebug_non_default_behavior_tlsmaxrsasize_events_total",
	"go_godebug_non_default_behavior_x509sha1_events_total",
	"go_godebug_non_default_behavior_x509usefallbackroots_events_total",
	"go_godebug_non_default_behavior_zipinsecurepath_events_total",
	"go_goroutines",
	"go_info",
	"go_memory_classes_heap_free_bytes",
	"go_memory_classes_heap_objects_bytes",
	"go_memory_classes_heap_released_bytes",
	"go_memory_classes_heap_stacks_bytes",
	"go_memory_classes_heap_unused_bytes",
	"go_memory_classes_metadata_mcache_free_bytes",
	"go_memory_classes_metadata_mcache_inuse_bytes",
	"go_memory_classes_metadata_mspan_free_bytes",
	"go_memory_classes_metadata_mspan_inuse_bytes",
	"go_memory_classes_metadata_other_bytes",
	"go_memory_classes_os_stacks_bytes",
	"go_memory_classes_other_bytes",
	"go_memory_classes_profiling_buckets_bytes",
	"go_memory_classes_total_bytes",
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
	"go_sync_mutex_wait_total_seconds_total",
	"go_threads",
}

func TestMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(baseplateGoCollectors))

	result, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	got := make(map[string]bool)
	for _, r := range result {
		got[r.GetName()] = true
	}

	for _, want := range expectedMetrics {
		if !got[want] {
			t.Errorf("want metric %q does not exist in got", want)
		}
	}
}
