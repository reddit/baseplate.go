package httpbp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/edgecontext"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	testTimeout = time.Millisecond * 100

	// copied from https://github.com/reddit/baseplate.py/blob/865ce3e19c549983b383dd49f748599929aab2b5/tests/__init__.py#L55
	headerWithValidAuth = "\x0c\x00\x01\x0b\x00\x01\x00\x00\x00\x0bt2_deadbeef\n\x00\x02\x00\x00\x00\x00\x00\x01\x86\xa0\x00\x0c\x00\x02\x0b\x00\x01\x00\x00\x00\x08beefdead\x00\x0b\x00\x03\x00\x00\x01\xaeeyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0Ml9leGFtcGxlIiwiZXhwIjoyNTI0NjA4MDAwfQ.dRzzfc9GmzyqfAbl6n_C55JJueraXk9pp3v0UYXw0ic6W_9RVa7aA1zJWm7slX9lbuYldwUtHvqaSsOpjF34uqr0-yMoRDVpIrbkwwJkNuAE8kbXGYFmXf3Ip25wMHtSXn64y2gJN8TtgAAnzjjGs9yzK9BhHILCDZTtmPbsUepxKmWTiEX2BdurUMZzinbcvcKY4Rb_Fl0pwsmBJFs7nmk5PvTyC6qivCd8ZmMc7dwL47mwy_7ouqdqKyUEdLoTEQ_psuy9REw57PRe00XCHaTSTRDCLmy4gAN6J0J056XoRHLfFcNbtzAmqmtJ_D9HGIIXPKq-KaggwK9I4qLX7g\x0c\x00\x04\x0b\x00\x01\x00\x00\x00$becc50f6-ff3d-407a-aa49-fa49531363be\x00\x00"
)

func TestWrap(t *testing.T) {
	t.Parallel()

	c := &counter{}
	if c.count != 0 {
		t.Fatal("Unexpected initial count.")
	}
	handler := httpbp.Wrap(
		"test",
		func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			return nil
		},
		testMiddleware(c),
	)
	handler(context.Background(), nil, nil)
	if c.count != 1 {
		t.Fatalf("Unexpected count value %v", c.count)
	}
}

func TestInjectServerSpan(t *testing.T) {
	t.Parallel()

	defer func() {
		tracing.CloseTracer()
		tracing.InitGlobalTracer(tracing.TracerConfig{})
	}()
	mmq := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	logger, startFailing := tracing.TestWrapper(t)
	tracing.InitGlobalTracer(tracing.TracerConfig{
		SampleRate:               1,
		MaxRecordTimeout:         testTimeout,
		Logger:                   logger,
		TestOnlyMockMessageQueue: mmq,
	})
	startFailing()

	req := newRequest(t)

	cases := []struct {
		name           string
		truster        httpbp.HeaderTrustHandler
		err            error
		hasAnnotations bool
	}{
		{
			name:           "trust/no-err",
			truster:        httpbp.AlwaysTrustHeaders{},
			hasAnnotations: true,
		},
		{
			name:           "trust/err",
			truster:        httpbp.AlwaysTrustHeaders{},
			hasAnnotations: true,
			err:            errors.New("test"),
		},
		{
			name:           "no-trust/no-err",
			truster:        httpbp.NeverTrustHeaders{},
			hasAnnotations: false,
		},
		{
			name:           "no-trust/err",
			truster:        httpbp.NeverTrustHeaders{},
			hasAnnotations: false,
			err:            errors.New("test"),
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				handle := httpbp.Wrap(
					"test",
					newTestHandler(testHandlerPlan{err: c.err}),
					httpbp.InjectServerSpan(c.truster),
				)
				handle(req.Context(), httptest.NewRecorder(), req)

				ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
				defer cancel()
				msg, err := mmq.Receive(ctx)
				if !c.hasAnnotations && err == nil {
					t.Fatal("expected error, got nil")
				} else if c.hasAnnotations && err != nil {
					t.Fatal(err)
				}
				if !c.hasAnnotations {
					return
				}

				var trace tracing.ZipkinSpan
				err = json.Unmarshal(msg, &trace)
				if err != nil {
					t.Fatal(err)
				}
				if len(trace.BinaryAnnotations) == 0 {
					t.Fatal("no binary annotations")
				}
				t.Logf("%#v", trace.BinaryAnnotations)
				if c.err != nil {
					hasError := false
					for _, annotation := range trace.BinaryAnnotations {
						if annotation.Key == "error" {
							hasError = true
						}
					}
					if !hasError {
						t.Error("error binary annotation was not present.")
					}
				}
			},
		)
	}
}

func TestInjectEdgeRequestContext(t *testing.T) {
	t.Parallel()

	const expectedID = "t2_example"

	store, dir := newSecretsStore(t)
	defer os.RemoveAll(dir)
	defer store.Close()

	impl := edgecontext.Init(edgecontext.Config{Store: store})
	req := newRequest(t)
	noHeader := newRequest(t)
	noHeader.Header.Del(httpbp.EdgeContextHeader)

	cases := []struct {
		name       string
		truster    httpbp.HeaderTrustHandler
		request    *http.Request
		expectedID string
	}{
		{
			name:       "trust/header",
			truster:    httpbp.AlwaysTrustHeaders{},
			request:    req,
			expectedID: expectedID,
		},
		{
			name:       "trust/no-header",
			truster:    httpbp.AlwaysTrustHeaders{},
			request:    noHeader,
			expectedID: "",
		},
		{
			name:       "no-trust",
			truster:    httpbp.NeverTrustHeaders{},
			request:    req,
			expectedID: "",
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				recorder := edgecontextRecorder{}
				handle := httpbp.Wrap(
					"test",
					newTestHandler(testHandlerPlan{}),
					httpbp.InjectEdgeRequestContext(
						c.truster,
						impl,
						log.TestWrapper(t),
					),
					edgecontextRecorderMiddleware(&recorder),
				)
				handle(c.request.Context(), httptest.NewRecorder(), c.request)

				if c.expectedID != "" {
					if recorder.EdgeContext == nil {
						t.Fatal("edge request context not set")
					}

					userID, ok := recorder.EdgeContext.User().ID()
					if !ok {
						t.Error("user should be logged in")
					}
					if userID != c.expectedID {
						t.Errorf("user ID mismatch, expected %q, got %q", c.expectedID, userID)
					}
				} else {
					if recorder.EdgeContext != nil {
						t.Fatal("edge request context should not be set")
					}
				}
			},
		)
	}
}
