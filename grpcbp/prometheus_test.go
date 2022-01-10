package grpcbp

import (
	"testing"
)

func TestServiceAndMethodSlug(t *testing.T) {
	testCases := []struct {
		name        string
		fullMethod  string
		wantService string
		wantMethod  string
	}{
		{
			name:        "success",
			fullMethod:  "/package.service/method",
			wantService: "package.service",
			wantMethod:  "method",
		},
		{
			name:        "success 2",
			fullMethod:  "package.service/method",
			wantService: "package.service",
			wantMethod:  "method",
		},
		{
			name:        "extra /",
			fullMethod:  "/foo/bar/baz",
			wantService: "foo",
			wantMethod:  "bar/baz",
		},
		{
			name:        "no /",
			fullMethod:  "package.service.method",
			wantService: "",
			wantMethod:  "package.service.method",
		},
		{
			name:        "empty input",
			fullMethod:  "",
			wantService: "",
			wantMethod:  "",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			gotService, gotMethod := serviceAndMethodSlug(tt.fullMethod)
			if got, want := gotService, tt.wantService; got != want {
				t.Errorf("serviceAndMethodSlug(%q).service = %v, want %v", tt.fullMethod, got, want)
			}
			if got, want := gotMethod, tt.wantMethod; got != want {
				t.Errorf("serviceAndMethodSlug(%q).method = %v, want %v", tt.fullMethod, got, want)
			}
		})
	}
}
