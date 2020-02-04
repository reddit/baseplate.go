package edgecontext_test

import (
	"context"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/thriftbp"
)

func getThriftContext() context.Context {
	return thrift.SetHeader(
		context.Background(),
		thriftbp.HeaderEdgeRequest,
		headerWithValidAuth,
	)
}

func getHTTPContext() context.Context {
	return httpbp.SetHeader(
		context.Background(),
		httpbp.EdgeContextContextKey,
		headerWithValidAuth,
	)
}

func TestInitializeEdgeContext(t *testing.T) {
	t.Parallel()

	expected, err := edgecontext.New(
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

	cases := []struct {
		name     string
		factory  edgecontext.ContextFactory
		ctx      context.Context
		expected *edgecontext.EdgeRequestContext
		ok       bool
	}{
		{
			name:     "http factory, thrift context",
			factory:  edgecontext.FromHTTPContext,
			ctx:      getThriftContext(),
			expected: nil,
			ok:       false,
		},
		{
			name:     "http factory, http context",
			factory:  edgecontext.FromHTTPContext,
			ctx:      getHTTPContext(),
			expected: expected,
			ok:       true,
		},
		{
			name:     "thrift factory, thrift context",
			factory:  edgecontext.FromThriftContext,
			ctx:      getThriftContext(),
			expected: expected,
			ok:       true,
		},
		{
			name:     "thrift factory, http context",
			factory:  edgecontext.FromThriftContext,
			ctx:      getHTTPContext(),
			expected: nil,
			ok:       false,
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()

				ctx := edgecontext.InitializeEdgeContext(c.ctx, nil, c.factory)
				ec, ok := edgecontext.GetEdgeContext(ctx)
				if ok != c.ok {
					t.Errorf("Ok does not match, expected %v got %v", c.ok, ok)
				}
				if c.expected == nil {
					if ec != nil {
						t.Errorf("Expected nil edge context, got %v", ec)
					}
				} else {
					if ec == nil {
						t.Fatalf("Unexpected nil edge context.")
					}
					if ec.SessionID() != c.expected.SessionID() {
						t.Errorf(
							"Expected session id %q, got %q",
							c.expected.SessionID(),
							ec.SessionID(),
						)
					}

					t.Run(
						"user",
						func(t *testing.T) {
							user := ec.User()
							expectedUser := c.expected.User()

							if user.IsLoggedIn() != expectedUser.IsLoggedIn() {
								t.Errorf("IsLoggedIn() did not match, got %v", user.IsLoggedIn())
							}

							userID, ok := user.ID()
							if !ok {
								t.Error("Failed to get user id")
							}
							expectedUserID, ok := expectedUser.ID()
							if !ok {
								t.Error("Failed to get expected user id")
							}
							if userID != expectedUserID {
								t.Errorf("Expected user id %q, got %q", expectedUserID, userID)
							}

							loid, ok := user.LoID()
							if !ok {
								t.Error("Failed to get loid from user")
							}
							expectedLoID, ok := expectedUser.LoID()
							if !ok {
								t.Error("Failed to get loid from expected user")
							}
							if loid != expectedLoID {
								t.Errorf("LoID expected %q, got %q", expectedLoID, loid)
							}

							ts, ok := user.CookieCreatedAt()
							if !ok {
								t.Error("Failed to get cookie created time from user")
							}
							expectedTS, ok := expectedUser.CookieCreatedAt()
							if !ok {
								t.Error("Failed to get cookie created time from expected user")
							}
							if !expectedTS.Equal(ts) {
								t.Errorf(
									"Expected cookie create timestamp %v, got %v",
									expectedTS,
									ts,
								)
							}

							if len(user.Roles()) != 0 {
								t.Errorf("Expected empty roles, got %+v", user.Roles())
							}
						},
					)
				}
			},
		)
	}
}

func TestInitializeHTTPEdgeContext(t *testing.T) {
	cases := []struct {
		name string
		ctx  context.Context
		// We only bother testing `ok` because InitializeHTTPEdgeContext just
		// calls InitializeEdgeContext which we test in more detail above.  This
		// test is just a sanity check to ensure we're using the right factory
		// method.
		ok bool
	}{
		{
			name: "thrift context",
			ctx:  getThriftContext(),
			ok:   false,
		},
		{
			name: "http context",
			ctx:  getHTTPContext(),
			ok:   true,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()

				ctx := edgecontext.InitializeHTTPEdgeContext(c.ctx, nil)
				if _, ok := edgecontext.GetEdgeContext(ctx); ok != c.ok {
					t.Errorf("Ok does not match, expected %v got %v", c.ok, ok)
				}
			},
		)
	}
}

func TestInitializeThriftEdgeContext(t *testing.T) {
	cases := []struct {
		name string
		ctx  context.Context
		// We only bother testing `ok` because InitializeThriftEdgeContext just
		// calls InitializeEdgeContext which we test in more detail above.  This
		// test is just a sanity check to ensure we're using the right factory
		// method.
		ok bool
	}{
		{
			name: "thrift context",
			ctx:  getThriftContext(),
			ok:   true,
		},
		{
			name: "http context",
			ctx:  getHTTPContext(),
			ok:   false,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()

				ctx := edgecontext.InitializeThriftEdgeContext(c.ctx, nil)
				if _, ok := edgecontext.GetEdgeContext(ctx); ok != c.ok {
					t.Errorf("Ok does not match, expected %v got %v", c.ok, ok)
				}
			},
		)
	}
}
