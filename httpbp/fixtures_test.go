package httpbp_test

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/secrets"
)

type jsonResponseBody struct {
	X int `json:"x"`
}

// pubkey copied from https://github.com/reddit/baseplate.py/blob/db9c1d7cddb1cb242546349e821cad0b0cbd6fce/tests/__init__.py#L12
const (
	secretStore = `{
	"secrets": {
		"secret/authentication/public-key": {
			"type": "versioned",
			"current": "foobar",
			"previous": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtzMnDEQPd75QZByogNlB\nNY2auyr4sy8UNTDARs79Edq/Jw5tb7ub412mOB61mVrcuFZW6xfmCRt0ILgoaT66\nTp1RpuEfghD+e7bYZ+Q2pckC1ZaVPIVVf/ZcCZ0tKQHoD8EpyyFINKjCh516VrCx\nKuOm2fALPB/xDwDBEdeVJlh5/3HHP2V35scdvDRkvr2qkcvhzoy0+7wUWFRZ2n6H\nTFrxMHQoHg0tutAJEkjsMw9xfN7V07c952SHNRZvu80V5EEpnKw/iYKXUjCmoXm8\ntpJv5kXH6XPgfvOirSbTfuo+0VGqVIx9gcomzJ0I5WfGTD22dAxDiRT7q7KZnNgt\nTwIDAQAB\n-----END PUBLIC KEY-----"
		},
		"secret/http/edge-context-signature": {
			"type": "versioned",
			"current": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU=",
			"previous": "aHVudGVyMg==",
			"encoding": "base64"
		},
		"secret/http/span-signature": {
			"type": "versioned",
			"current": "Y2RvVXhNMVdsTXJma3BDaHRGZ0dPYkVGSg==",
			"encoding": "base64"
		}
	},
	"vault": {
		"url": "vault.reddit.ue1.snooguts.net",
		"token": "17213328-36d4-11e7-8459-525400f56d04"
	}
}`
)

func newSecretsStore(t testing.TB) (store *secrets.Store, dir string) {
	dir, err := ioutil.TempDir("", "edge_context_test_")
	if err != nil {
		t.Fatal(err)
	}

	tmpFile, err := ioutil.TempFile(dir, "secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write([]byte(secretStore)); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = secrets.NewStore(context.Background(), tmpPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	return
}

type edgecontextRecorder struct {
	EdgeContext *edgecontext.EdgeRequestContext
}

func edgecontextRecorderMiddleware(recorder *edgecontextRecorder) httpbp.Middleware {
	return func(name string, next httpbp.HandlerFunc) httpbp.HandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			recorder.EdgeContext, _ = edgecontext.GetEdgeContext(ctx)
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
					w.Header().Add(k, v)
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

func newRequest(t *testing.T) *http.Request {
	req, err := http.NewRequest("get", "localhost:9090", strings.NewReader("test"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add(httpbp.EdgeContextHeader, headerWithValidAuth)
	req.Header.Add(httpbp.SpanSampledHeader, "1")
	return req
}
