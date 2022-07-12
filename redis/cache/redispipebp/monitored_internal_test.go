package redispipebp

import (
	"testing"
)

func TestExtractCommand(t *testing.T) {
	for _, c := range []struct {
		command string
		want    string
	}{
		{
			command: "",
			want:    "",
		},
		{
			command: "SET foo:x x",
			want:    "SET",
		},
		{
			command: " SET foo:x x",
			want:    "",
		},
		{
			command: "set  foo:x x",
			want:    "set",
		},
	} {
		t.Run(c.command, func(t *testing.T) {
			got := extractCommand(c.command)
			if got != c.want {
				t.Errorf("Command %q, got %q, want %q", c.command, got, c.want)
			}
		})
	}
}
