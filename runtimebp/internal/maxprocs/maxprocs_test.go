package maxprocs

import (
	"runtime"
	"testing"
)

func TestSet(t *testing.T) {
	sentinel := 99
	automaxprocsSentinel := 111

	origAutomaxprocs := setWithAutomaxprocs
	defer func() { setWithAutomaxprocs = origAutomaxprocs }()
	setWithAutomaxprocs = func() { runtime.GOMAXPROCS(automaxprocsSentinel) }

	for _, tt := range []struct {
		name           string
		env            map[string]string
		wantGOMAXPROCS int
	}{
		{
			name: "GOMAXPROCS",
			env: map[string]string{
				"GOMAXPROCS": "42",
			},
			wantGOMAXPROCS: sentinel, // since our package abdicates responsibility to Go
		},
		{
			name: "request_no_scale",
			env: map[string]string{
				"BASEPLATE_CPU_REQUEST": "42",
			},
			wantGOMAXPROCS: 63, // 42 * 1.5 scale default multiplier
		},
		{
			name: "request_with_scale",
			env: map[string]string{
				"BASEPLATE_CPU_REQUEST":       "42",
				"BASEPLATE_CPU_REQUEST_SCALE": "0.9",
			},
			wantGOMAXPROCS: 38, // ceil(42 * 0.9)
		},
		{
			name: "invalid_request",
			env: map[string]string{
				"BASEPLATE_CPU_REQUEST": "not a number",
			},
			wantGOMAXPROCS: automaxprocsSentinel,
		},
		{
			name: "invalid_scale",
			env: map[string]string{
				"BASEPLATE_CPU_REQUEST":       "42",
				"BASEPLATE_CPU_REQUEST_SCALE": "not a number",
			},
			wantGOMAXPROCS: 63, // 42 * 1.5 scale default multiplier
		},
		{
			name: "zero_request",
			env: map[string]string{
				"BASEPLATE_CPU_REQUEST": "0",
			},
			wantGOMAXPROCS: automaxprocsSentinel,
		},
		{
			name: "zero_scale",
			env: map[string]string{
				"BASEPLATE_CPU_REQUEST":       "42",
				"BASEPLATE_CPU_REQUEST_SCALE": "0",
			},
			wantGOMAXPROCS: 63, // 42 * 1.5 scale default multiplier
		},
		{
			name: "min_2",
			env: map[string]string{
				"BASEPLATE_CPU_REQUEST":       "1",
				"BASEPLATE_CPU_REQUEST_SCALE": "1",
			},
			wantGOMAXPROCS: 2,
		},
		{
			name: "gomaxprocs_and_request",
			env: map[string]string{
				"GOMAXPROCS":            "anything at all",
				"BASEPLATE_CPU_REQUEST": "12",
			},
			wantGOMAXPROCS: sentinel, // since our package abdicates responsibility to Go
		},
		{
			name:           "nothing_set",
			env:            map[string]string{},
			wantGOMAXPROCS: automaxprocsSentinel,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			orig := runtime.GOMAXPROCS(0)
			defer runtime.GOMAXPROCS(orig)
			// set GOMAXPROCS to a known value
			runtime.GOMAXPROCS(sentinel)

			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			Set()

			if got, want := runtime.GOMAXPROCS(0), tt.wantGOMAXPROCS; got != want {
				t.Errorf("got GOMAXPROCS=%d, want %d", got, want)
			}
		})
	}
}
