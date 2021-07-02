package configbp_test // to break dep cycle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/configbp"
	"github.com/reddit/baseplate.go/log"
)

func init() {
	log.InitLogger(log.DebugLevel)
}

func TestParseStrictFile(t *testing.T) {
	valueFromEnv := "_value_from_environment_var_"
	t.Setenv("VALUE_FROM_ENV", valueFromEnv)

	tests := []struct {
		desc    string
		content string
		target  interface{}
		want    interface{}
	}{
		{
			desc: "basic_env",
			content: `
addr: localhost:1234
log:
  level: debug
sentry:
  dsn: $VALUE_FROM_ENV
  serverName: ${VALUE_FROM_ENV}
`,
			target: &baseplate.Config{},
			want: &baseplate.Config{
				Addr: "localhost:1234",
				Log: log.Config{
					Level: log.DebugLevel,
				},
				Sentry: log.SentryConfig{
					DSN:        valueFromEnv,
					ServerName: valueFromEnv,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			dir := t.TempDir() // automatically cleaned up
			filename := filepath.Join(dir, "test.yaml")
			if err := os.WriteFile(filename, []byte(test.content), 0600); err != nil {
				t.Fatalf("SETUP: failed to write file: %s", err)
			}
			if err := configbp.ParseStrictFile(filename, test.target); err != nil {
				t.Fatalf("ParseStrictFile(%q): %s", filename, err)
			}
			if diff := cmp.Diff(test.target, test.want); diff != "" {
				t.Errorf("Parsed config incorrect: (-got +want)\n%s", diff)
			}
		})
	}
}
