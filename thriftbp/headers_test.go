package thriftbp_test

import (
	"context"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/thriftbp"
)

func TestAttachEdgeRequestContext(t *testing.T) {
	store := newSecretsStore(t)
	defer store.Close()

	impl := edgecontext.Init(edgecontext.Config{Store: store})
	ec, err := edgecontext.FromHeader(context.Background(), headerWithValidAuth, impl)
	if err != nil {
		t.Fatal(err)
	}

	ctx := thriftbp.AttachEdgeRequestContext(context.Background(), ec)
	headers := thrift.GetWriteHeaderList(ctx)
	var found bool
	for _, key := range headers {
		if key == thriftbp.HeaderEdgeRequest {
			found = true
			break
		}
	}
	if !found {
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

func TestAttachEdgeRequestContextNilHeader(t *testing.T) {
	ctx := thrift.SetWriteHeaderList(
		context.Background(),
		[]string{thriftbp.HeaderEdgeRequest},
	)
	ctx = thriftbp.AttachEdgeRequestContext(ctx, nil)

	_, ok := thrift.GetHeader(ctx, thriftbp.HeaderEdgeRequest)
	if ok {
		t.Fatal("header should not be set")
	}
}

func headerInWriteHeaderList(ctx context.Context, t *testing.T, header string) {
	t.Helper()

	headers := thrift.GetWriteHeaderList(ctx)
	for _, h := range headers {
		if h == header {
			return
		}
	}
	t.Errorf("Cannot find header %q in list %#v", header, headers)
}

func TestAddClientHeader(t *testing.T) {
	const (
		key      = "key"
		expected = "value"
	)
	ctx := thriftbp.AddClientHeader(context.Background(), key, expected)
	if value, ok := thrift.GetHeader(ctx, key); value != expected {
		t.Errorf("Expected header value to be %q, got %q, %v", expected, value, ok)
	}
	headerInWriteHeaderList(ctx, t, key)
}
