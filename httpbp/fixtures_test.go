package httpbp_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/ecinterface"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/secrets"
)

type jsonResponseBody struct {
	X int `json:"x"`
}

const (
	testTimeout = time.Millisecond * 100
)

var testSecrets = map[string]secrets.GenericSecret{
	"secret/http/edge-context-signature": {
		Type:     secrets.VersionedType,
		Current:  "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU=",
		Previous: "aHVudGVyMg==",
		Encoding: secrets.Base64Encoding,
	},
	"secret/http/span-signature": {
		Type:     secrets.VersionedType,
		Current:  "Y2RvVXhNMVdsTXJma3BDaHRGZ0dPYkVGSg==",
		Encoding: secrets.Base64Encoding,
	},
}

func newSecretsStore(t testing.TB) *secrets.Store {
	t.Helper()

	store, _, err := secrets.NewTestSecrets(context.Background(), testSecrets)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

type edgecontextRecorder struct {
	header string
}

func edgecontextRecorderMiddleware(impl ecinterface.Interface, recorder *edgecontextRecorder) httpbp.Middleware {
	return func(name string, next httpbp.HandlerFunc) httpbp.HandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			recorder.header, _ = impl.ContextToHeader(ctx)
			return next(ctx, w, r)
		}
	}
}

type testHandlerPlan struct {
	code    int
	headers http.Header
	cookies []*http.Cookie
	body    interface{}
	err     error
}

func newTestHandler(plan testHandlerPlan) httpbp.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		if plan.headers != nil {
			for k, values := range plan.headers {
				for _, v := range values {
					w.Header().Set(k, v)
				}
			}
		}

		for _, cookie := range plan.cookies {
			http.SetCookie(w, cookie)
		}
		if plan.err != nil {
			return plan.err
		}
		return httpbp.WriteJSON(
			w,
			httpbp.Response{
				Code: plan.code,
				Body: plan.body,
			},
		)
	}
}

type counter struct {
	count int
}

func (c *counter) Incr() {
	c.count++
}

func testMiddleware(c *counter) httpbp.Middleware {
	return func(name string, next httpbp.HandlerFunc) httpbp.HandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			c.Incr()
			return next(ctx, w, r)
		}
	}
}

func newRequest(t testing.TB, ecHeader string) *http.Request {
	t.Helper()

	req, err := http.NewRequest("get", "localhost:9090", strings.NewReader("test"))
	if err != nil {
		t.Fatal(err)
	}
	if ecHeader != "" {
		str := base64.StdEncoding.EncodeToString([]byte(ecHeader))
		req.Header.Set(httpbp.EdgeContextHeader, str)
	}
	req.Header.Set(httpbp.SpanSampledHeader, "1")
	return req
}
