package thriftclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/mqsend"
	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/set"
	"github.com/reddit/baseplate.go/thriftbp"
	"github.com/reddit/baseplate.go/thriftclient"
	"github.com/reddit/baseplate.go/tracing"
)

const (
	// copied from https://github.com/reddit/baseplate.py/blob/865ce3e19c549983b383dd49f748599929aab2b5/tests/__init__.py#L55
	headerWithValidAuth = "\x0c\x00\x01\x0b\x00\x01\x00\x00\x00\x0bt2_deadbeef\n\x00\x02\x00\x00\x00\x00\x00\x01\x86\xa0\x00\x0c\x00\x02\x0b\x00\x01\x00\x00\x00\x08beefdead\x00\x0b\x00\x03\x00\x00\x01\xaeeyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0Ml9leGFtcGxlIiwiZXhwIjoyNTI0NjA4MDAwfQ.dRzzfc9GmzyqfAbl6n_C55JJueraXk9pp3v0UYXw0ic6W_9RVa7aA1zJWm7slX9lbuYldwUtHvqaSsOpjF34uqr0-yMoRDVpIrbkwwJkNuAE8kbXGYFmXf3Ip25wMHtSXn64y2gJN8TtgAAnzjjGs9yzK9BhHILCDZTtmPbsUepxKmWTiEX2BdurUMZzinbcvcKY4Rb_Fl0pwsmBJFs7nmk5PvTyC6qivCd8ZmMc7dwL47mwy_7ouqdqKyUEdLoTEQ_psuy9REw57PRe00XCHaTSTRDCLmy4gAN6J0J056XoRHLfFcNbtzAmqmtJ_D9HGIIXPKq-KaggwK9I4qLX7g\x0c\x00\x04\x0b\x00\x01\x00\x00\x00$becc50f6-ff3d-407a-aa49-fa49531363be\x00\x00"

	method = "test_method"

	// pubkey copied from https://github.com/reddit/baseplate.py/blob/db9c1d7cddb1cb242546349e821cad0b0cbd6fce/tests/__init__.py#L12
	secretStore = `{
	"secrets": {
		"secret/authentication/public-key": {
			"type": "versioned",
			"current": "foobar",
			"previous": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtzMnDEQPd75QZByogNlB\nNY2auyr4sy8UNTDARs79Edq/Jw5tb7ub412mOB61mVrcuFZW6xfmCRt0ILgoaT66\nTp1RpuEfghD+e7bYZ+Q2pckC1ZaVPIVVf/ZcCZ0tKQHoD8EpyyFINKjCh516VrCx\nKuOm2fALPB/xDwDBEdeVJlh5/3HHP2V35scdvDRkvr2qkcvhzoy0+7wUWFRZ2n6H\nTFrxMHQoHg0tutAJEkjsMw9xfN7V07c952SHNRZvu80V5EEpnKw/iYKXUjCmoXm8\ntpJv5kXH6XPgfvOirSbTfuo+0VGqVIx9gcomzJ0I5WfGTD22dAxDiRT7q7KZnNgt\nTwIDAQAB\n-----END PUBLIC KEY-----"
		}
	},
	"vault": {
		"url": "vault.reddit.ue1.snooguts.net",
		"token": "17213328-36d4-11e7-8459-525400f56d04"
	}
}`
)

func initClients() (*thriftclient.MockClient, *thriftclient.RecordedClient, thrift.TClient) {
	mock := &thriftclient.MockClient{FailUnregisteredMethods: true}
	recorder := thriftclient.NewRecordedClient(mock)
	client := thriftclient.Wrap(recorder, thriftclient.BaseplateDefaultMiddlewares()...)
	return mock, recorder, client
}

func initServerSpan(ctx context.Context, t *testing.T) (context.Context, *mqsend.MockMessageQueue) {
	t.Helper()

	recorder := mqsend.OpenMockMessageQueue(mqsend.MessageQueueConfig{
		MaxQueueSize:   100,
		MaxMessageSize: 1024,
	})
	tracing.InitGlobalTracer(tracing.TracerConfig{
		SampleRate:               1.0,
		TestOnlyMockMessageQueue: recorder,
	})

	span, ctx := opentracing.StartSpanFromContext(
		ctx,
		"test-service",
		tracing.SpanTypeOption{Type: tracing.SpanTypeServer},
	)
	tracing.AsSpan(span).SetDebug(true)
	return ctx, recorder
}

func initLocalSpan(ctx context.Context, t *testing.T) (context.Context, *mqsend.MockMessageQueue) {
	t.Helper()

	ctx, recorder := initServerSpan(ctx, t)
	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		t.Fatal("server span was nill")
	}
	_, ctx = opentracing.StartSpanFromContext(
		ctx,
		"local-test",
		tracing.LocalComponentOption{Name: ""},
	)
	return ctx, recorder
}

func drainRecorder(recorder *mqsend.MockMessageQueue) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	return recorder.Receive(ctx)
}

func newSecretsStore(t testing.TB) (store *secrets.Store, dir string) {
	dir, err := ioutil.TempDir("", "thriftclient_tests_")
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

func TestWrapMonitoredClient(t *testing.T) {
	cases := []struct {
		name          string
		call          thriftclient.MockCall
		errorExpected bool
		initSpan      func(context.Context, *testing.T) (context.Context, *mqsend.MockMessageQueue)
	}{
		{
			name: "server span: success",
			call: func(ctx context.Context, args, result thrift.TStruct) error {
				return nil
			},
			errorExpected: false,
			initSpan:      initServerSpan,
		},
		{
			name: "server span: error",
			call: func(ctx context.Context, args, result thrift.TStruct) error {
				return errors.New("test error")
			},
			errorExpected: true,
			initSpan:      initServerSpan,
		},
		{
			name: "local span: success",
			call: func(ctx context.Context, args, result thrift.TStruct) error {
				return nil
			},
			errorExpected: false,
			initSpan:      initLocalSpan,
		},
		{
			name: "local span: error",
			call: func(ctx context.Context, args, result thrift.TStruct) error {
				return errors.New("test error")
			},
			errorExpected: true,
			initSpan:      initLocalSpan,
		},
	}
	for _, c := range cases {
		t.Run(
			c.name,
			func(t *testing.T) {
				defer func() {
					tracing.CloseTracer()
					tracing.InitGlobalTracer(tracing.TracerConfig{})
				}()

				mock, recorder, client := initClients()
				mock.AddMockCall(method, c.call)

				ctx, mmq := c.initSpan(context.Background(), t)
				if err := client.Call(ctx, method, nil, nil); !c.errorExpected && err != nil {
					t.Fatal(err)
				} else if c.errorExpected && err == nil {
					t.Fatal("expected an error, got nil")
				}
				call := recorder.Calls()[0]
				s := opentracing.SpanFromContext(call.Ctx)
				if s == nil {
					t.Fatal("span was nil")
				}
				span := tracing.AsSpan(s)
				if span.Name() != method {
					t.Errorf("span name mismatch, expected %q, got %q", method, span.Name())
				}
				if span.SpanType() != tracing.SpanTypeClient {
					t.Errorf("span type mismatch, expected %s, got %s", tracing.SpanTypeClient, span.SpanType())
				}
				if call.Method != method {
					t.Errorf("method mismatch, expected %q, got %q", method, call.Method)
				}

				msg, err := drainRecorder(mmq)
				if err != nil {
					t.Fatal(err)
				}

				var trace tracing.ZipkinSpan
				err = json.Unmarshal(msg, &trace)
				if err != nil {
					t.Fatal(err)
				}
				hasError := false
				for _, annotation := range trace.BinaryAnnotations {
					if annotation.Key == "error" {
						hasError = true
					}
				}
				if !c.errorExpected && hasError {
					t.Error("error binary annotation present")
				} else if c.errorExpected && !hasError {
					t.Error("error binary annotation not present")
				}
			},
		)
	}
}

func TestForwardEdgeRequestContext(t *testing.T) {
	store, dir := newSecretsStore(t)
	defer os.RemoveAll(dir)
	defer store.Close()

	impl := edgecontext.Init(edgecontext.Config{Store: store})
	ec, err := edgecontext.FromHeader(headerWithValidAuth, impl)
	if err != nil {
		t.Fatal(err)
	}

	ctx := thrift.SetHeader(
		context.Background(),
		thriftbp.HeaderEdgeRequest,
		headerWithValidAuth,
	)
	ctx = thriftbp.InitializeEdgeContext(ctx, impl)

	mock, recorder, client := initClients()
	mock.AddMockCall(
		method,
		func(ctx context.Context, args, result thrift.TStruct) error {
			return nil
		},
	)

	if err := client.Call(ctx, method, nil, nil); err != nil {
		t.Fatal(err)
	}

	if len(recorder.Calls()) != 1 {
		t.Fatalf("wrong number of calls: %d", len(recorder.Calls()))
	}

	ctx = recorder.Calls()[0].Ctx
	headers := set.StringSliceToSet(thrift.GetWriteHeaderList(ctx))
	if !headers.Contains(thriftbp.HeaderEdgeRequest) {
		t.Error("header not added to thrift write list")
	}

	header, ok := thrift.GetHeader(ctx, thriftbp.HeaderEdgeRequest)
	if !ok {
		t.Fatal("header not set")
	}
	if header != ec.Header() {
		t.Errorf("header mismatch, expected %q, got %q", ec.Header(), header)
	}
}

func TestForwardEdgeRequestContextNotSet(t *testing.T) {
	mock, recorder, client := initClients()
	mock.AddMockCall(
		method,
		func(ctx context.Context, args, result thrift.TStruct) error {
			return nil
		},
	)

	if err := client.Call(context.Background(), method, nil, nil); err != nil {
		t.Fatal(err)
	}

	if len(recorder.Calls()) != 1 {
		t.Fatalf("wrong number of calls: %d", len(recorder.Calls()))
	}

	ctx := recorder.Calls()[0].Ctx
	headers := set.StringSliceToSet(thrift.GetWriteHeaderList(ctx))
	if headers.Contains(thriftbp.HeaderEdgeRequest) {
		t.Error("header should not be added to thrift write list")
	}

	_, ok := thrift.GetHeader(ctx, thriftbp.HeaderEdgeRequest)
	if ok {
		t.Fatal("header should not be set")
	}
}
