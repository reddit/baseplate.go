package thriftbp_test

import (
	"context"
	"os"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/set"
	"github.com/reddit/baseplate.go/thriftbp"
)

func TestAttachEdgeRequestContext(t *testing.T) {
	store, dir := newSecretsStore(t)
	defer os.RemoveAll(dir)
	defer store.Close()

	impl := edgecontext.Init(edgecontext.Config{Store: store})
	ec, err := edgecontext.FromHeader(headerWithValidAuth, impl)
	if err != nil {
		t.Fatal(err)
	}

	ctx := thriftbp.AttachEdgeRequestContext(context.Background(), ec)
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

func TestAttachEdgeRequestContextNilHeader(t *testing.T) {
	ctx := thrift.SetWriteHeaderList(
		context.Background(),
		[]string{thriftbp.HeaderEdgeRequest},
	)
	ctx = thriftbp.AttachEdgeRequestContext(ctx, nil)

	headers := set.StringSliceToSet(thrift.GetWriteHeaderList(ctx))
	if headers.Contains(thriftbp.HeaderEdgeRequest) {
		t.Error("header was not removed from the thrift write list")
	}

	_, ok := thrift.GetHeader(ctx, thriftbp.HeaderEdgeRequest)
	if ok {
		t.Fatal("header should not be set")
	}
}
