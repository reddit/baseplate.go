package httpbp_test

import (
	"net/url"
	"testing"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
)

func TestGetHealthCheckProbe(t *testing.T) {
	for _, _c := range []struct {
		name      string
		code      string
		shouldErr bool
		expected  int64
	}{
		{
			name:     "empty",
			expected: int64(baseplate.IsHealthyProbe_READINESS),
		},
		{
			name:     "number",
			code:     "2",
			expected: 2,
		},
		{
			name:     "large-number",
			code:     "999",
			expected: 999,
		},
		{
			name:     "negative-number",
			code:     "-1",
			expected: -1,
		},
		{
			name:     "string",
			code:     "startup",
			expected: int64(baseplate.IsHealthyProbe_STARTUP),
		},
		{
			name:     "string-mixed-case",
			code:     "lIvEnEsS",
			expected: int64(baseplate.IsHealthyProbe_LIVENESS),
		},
		{
			name:      "unknown",
			code:      "hello world",
			shouldErr: true,
			expected:  int64(baseplate.IsHealthyProbe_READINESS),
		},
	} {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()

				var query url.Values
				if c.code != "" {
					query = url.Values{
						httpbp.HealthCheckProbeQuery: []string{
							c.code,
						},
					}
				}
				probe, err := httpbp.GetHealthCheckProbe(query)
				if probe != c.expected {
					t.Errorf("Expected probe value %d, got %d", c.expected, probe)
				}
				if err != nil && !c.shouldErr {
					t.Errorf("Did not expect error, got %v", err)
				}
				if err == nil && c.shouldErr {
					t.Error("Expected error, got nil")
				}
			},
		)
	}
}
