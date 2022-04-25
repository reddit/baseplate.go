package log

import (
	"runtime/debug"
	"testing"
)

func TestGetVersionFromBuildInfo(t *testing.T) {
	for _, c := range []struct {
		label string
		want  string
		info  *debug.BuildInfo
	}{
		{
			label: "normal",
			want:  "deadbeef",
			info: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{
						Key:   "vcs.revision",
						Value: "deadbeef",
					},
					{
						Key:   "vcs.modified",
						Value: "false",
					},
				},
			},
		},
		{
			label: "reverse-order",
			want:  "deadbeef",
			info: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{
						Key:   "vcs.modified",
						Value: "false",
					},
					{
						Key:   "vcs.revision",
						Value: "deadbeef",
					},
				},
			},
		},
		{
			label: "dirty",
			want:  "deadbeef-dirty",
			info: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{
						Key:   "vcs.revision",
						Value: "deadbeef",
					},
					{
						Key:   "vcs.modified",
						Value: "true",
					},
				},
			},
		},
		{
			label: "reverse-dirty",
			want:  "deadbeef-dirty",
			info: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{
						Key:   "vcs.modified",
						Value: "true",
					},
					{
						Key:   "vcs.revision",
						Value: "deadbeef",
					},
				},
			},
		},
		{
			label: "revision-over-tag",
			want:  "deadbeef",
			info: &debug.BuildInfo{
				Main: debug.Module{
					Version: "v1.0",
				},
				Settings: []debug.BuildSetting{
					{
						Key:   "vcs.revision",
						Value: "deadbeef",
					},
				},
			},
		},
		{
			label: "tag-only",
			want:  "v1.0",
			info: &debug.BuildInfo{
				Main: debug.Module{
					Version: "v1.0",
				},
			},
		},
		{
			label: "untagged-no-fallback",
			want:  "",
			info: &debug.BuildInfo{
				Main: debug.Module{
					Version: "(devel)",
				},
			},
		},
	} {
		t.Run(c.label, func(t *testing.T) {
			got := getVersionFromBuildInfo(c.info)
			if got != c.want {
				t.Errorf("got: %q, want: %q", got, c.want)
			}
		})
	}
}
