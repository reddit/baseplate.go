package edgecontext_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/reddit/baseplate.go/httpbp"

	"github.com/reddit/baseplate.go/edgecontext"
)

func TestAttachHTTPHeader(t *testing.T) {
	t.Parallel()

	e, err := edgecontext.New(
		context.Background(),
		edgecontext.NewArgs{
			LoID:          expectedLoID,
			LoIDCreatedAt: expectedCookieTime,
			SessionID:     expectedSessionID,
			AuthToken:     validToken,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Run(
		"AttachHTTPHeader",
		func(t *testing.T) {
			headers := make(http.Header)
			e.AttachHTTPHeader(headers)
			h := headers.Get(httpbp.EdgeContextHeader)
			if h == "" {
				t.Fatal("Header was not attached.")
			}
			ctx := httpbp.SetHeader(context.Background(), httpbp.EdgeContextContextKey, h)
			ec, err := edgecontext.FromHTTPContext(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(e, ec) {
				t.Fatalf("Expected %#v, got %#v", e, ec)
			}
		},
	)
}
