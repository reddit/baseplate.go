package edgecontext_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/thriftbp"

	"github.com/apache/thrift/lib/go/thrift"
)

const (
	// copied from https://github.com/reddit/baseplate.py/blob/db9c1d7cddb1cb242546349e821cad0b0cbd6fce/tests/__init__.py#L56
	headerWithNoAuthNoDevice = "\x0c\x00\x01\x0b\x00\x01\x00\x00\x00\x0bt2_deadbeef\n\x00\x02\x00\x00\x00\x00\x00\x01\x86\xa0\x00\x0c\x00\x02\x0b\x00\x01\x00\x00\x00\x08beefdead\x00\x00"

	// copied from https://github.com/reddit/baseplate.py/blob/865ce3e19c549983b383dd49f748599929aab2b5/tests/__init__.py#L55-L59
	headerWithNoAuth      = "\x0c\x00\x01\x0b\x00\x01\x00\x00\x00\x0bt2_deadbeef\n\x00\x02\x00\x00\x00\x00\x00\x01\x86\xa0\x00\x0c\x00\x02\x0b\x00\x01\x00\x00\x00\x08beefdead\x00\x0c\x00\x04\x0b\x00\x01\x00\x00\x00$becc50f6-ff3d-407a-aa49-fa49531363be\x00\x00"
	headerWithValidAuth   = "\x0c\x00\x01\x0b\x00\x01\x00\x00\x00\x0bt2_deadbeef\n\x00\x02\x00\x00\x00\x00\x00\x01\x86\xa0\x00\x0c\x00\x02\x0b\x00\x01\x00\x00\x00\x08beefdead\x00\x0b\x00\x03\x00\x00\x01\xaeeyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0Ml9leGFtcGxlIiwiZXhwIjoyNTI0NjA4MDAwfQ.dRzzfc9GmzyqfAbl6n_C55JJueraXk9pp3v0UYXw0ic6W_9RVa7aA1zJWm7slX9lbuYldwUtHvqaSsOpjF34uqr0-yMoRDVpIrbkwwJkNuAE8kbXGYFmXf3Ip25wMHtSXn64y2gJN8TtgAAnzjjGs9yzK9BhHILCDZTtmPbsUepxKmWTiEX2BdurUMZzinbcvcKY4Rb_Fl0pwsmBJFs7nmk5PvTyC6qivCd8ZmMc7dwL47mwy_7ouqdqKyUEdLoTEQ_psuy9REw57PRe00XCHaTSTRDCLmy4gAN6J0J056XoRHLfFcNbtzAmqmtJ_D9HGIIXPKq-KaggwK9I4qLX7g\x0c\x00\x04\x0b\x00\x01\x00\x00\x00$becc50f6-ff3d-407a-aa49-fa49531363be\x00\x00"
	headerWithExpiredAuth = "\x0c\x00\x01\x0b\x00\x01\x00\x00\x00\x0bt2_deadbeef\n\x00\x02\x00\x00\x00\x00\x00\x01\x86\xa0\x00\x0c\x00\x02\x0b\x00\x01\x00\x00\x00\x08beefdead\x00\x0b\x00\x03\x00\x00\x01\xaeeyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0Ml9leGFtcGxlIiwiZXhwIjoxMjYyMzA0MDAwfQ.iUD0J2blW-HGtH86s66msBXymCRCgyxAZJ6xX2_SXD-kegm-KjOlIemMWFZtsNv9DJI147cNP81_gssewvUnhIHLVvXWCTOROasXbA9Yf2GUsjxoGSB7474ziPOZquAJKo8ikERlhOOVk3r4xZIIYCuc4vGZ7NfqFxjDGKAWj5Tt4VUiWXK1AdxQck24GyNOSXs677vIJnoD8EkgWqNuuwY-iFOAPVcoHmEuzhU_yUeQnY8D-VztJkip5-YPEnuuf-dTSmPbdm9ZTOP8gjTsG0Sdvb9NdLId0nEwawRy8CfFEGQulqHgd1bqTm25U-NyXQi7zroi1GEdykZ3w9fVNQ\x00"
	headerWithAnonAuth    = "\x0c\x00\x01\x0b\x00\x01\x00\x00\x00\x0bt2_deadbeef\n\x00\x02\x00\x00\x00\x00\x00\x01\x86\xa0\x00\x0c\x00\x02\x0b\x00\x01\x00\x00\x00\x08beefdead\x00\x0b\x00\x03\x00\x00\x01\xc0eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlcyI6WyJhbm9ueW1vdXMiXSwic3ViIjpudWxsLCJleHAiOjI1MjQ2MDgwMDB9.gQDiVzOUh70mKKK-YBTnLHWBOEuQyRllEE1-EIMfy3x5K8PsH9FB6Oy9S5HbILjfGFNrIBeux9HyW6hBDikoZDhn5QWyPNitL1pzMNONGGrXzSfaDoDbFy4MLD03A7zjG3qWBn_wLjgzUXX6qVX6W_gWO7dMqrq0iFvEegue-xQ1HGiXfPgnTrXRRovUO3JHy1LcZsmOjltYj5VGUTWXodBM8ObKEealDxg8yskEPy0IuujNMmb9eIyuHB8Ozzpg-lr790lxP37s5HCf18vrZ-IhRmLcLCqm5WSFyq_Ld2ByblBKL9pPst1AZYZTXNRIqovTAqr6v0-xjUeJ1iho9A\x00"
)

const (
	expectedLoID      = "t2_deadbeef"
	expectedSessionID = "beefdead"
	expectedDeviceID  = "becc50f6-ff3d-407a-aa49-fa49531363be"
)

var expectedCookieTime = time.Unix(100, 0)

func TestNew(t *testing.T) {
	ctx := context.Background()
	e, err := edgecontext.New(
		ctx,
		globalTestImpl,
		edgecontext.NewArgs{
			LoID:          expectedLoID,
			LoIDCreatedAt: expectedCookieTime,
			SessionID:     expectedSessionID,
			AuthToken:     validToken,
			DeviceID:      expectedDeviceID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx = e.AttachToContext(ctx)
	if header, ok := thrift.GetHeader(ctx, thriftbp.HeaderEdgeRequest); !ok {
		t.Error("Failed to attach header to thrift context")
	} else {
		if header != headerWithValidAuth {
			t.Errorf("Header expected %q, got %q", headerWithValidAuth, header)
		}
	}
}

func TestFromThriftContext(t *testing.T) {
	const expectedUser = "t2_example"

	t.Run(
		"no-header",
		func(t *testing.T) {
			ctx := context.Background()
			_, err := edgecontext.FromThriftContext(ctx, globalTestImpl)
			if !errors.Is(err, edgecontext.ErrNoHeader) {
				t.Errorf("Expected ErrNoHeader, got %v", err)
			}
		},
	)

	t.Run(
		"no-device-id",
		func(t *testing.T) {
			ctx := thrift.SetHeader(
				context.Background(),
				thriftbp.HeaderEdgeRequest,
				headerWithNoAuthNoDevice,
			)
			e, err := edgecontext.FromThriftContext(ctx, globalTestImpl)
			if err != nil {
				t.Fatal(err)
			}

			if e.DeviceID() != "" {
				t.Errorf("Unexpected device id %q", e.DeviceID())
			}
		},
	)

	t.Run(
		"no-auth",
		func(t *testing.T) {
			ctx := thrift.SetHeader(
				context.Background(),
				thriftbp.HeaderEdgeRequest,
				headerWithNoAuth,
			)
			e, err := edgecontext.FromThriftContext(ctx, globalTestImpl)
			if err != nil {
				t.Fatal(err)
			}

			if e.SessionID() != expectedSessionID {
				t.Errorf(
					"Expected session id %q, got %q",
					expectedSessionID,
					e.SessionID(),
				)
			}

			if e.DeviceID() != expectedDeviceID {
				t.Errorf(
					"Expected device id %q, got %q",
					expectedDeviceID,
					e.DeviceID(),
				)
			}

			t.Run(
				"user",
				func(t *testing.T) {
					user := e.User()
					if user.IsLoggedIn() {
						t.Error("Expected logged out user, IsLoggedIn() returned true")
					}
					if loid, ok := user.LoID(); !ok {
						t.Error("Failed to get loid from user")
					} else {
						if loid != expectedLoID {
							t.Errorf("LoID expected %q, got %q", expectedLoID, loid)
						}
					}
					if ts, ok := user.CookieCreatedAt(); !ok {
						t.Error("Failed to get cookie created time from user")
					} else {
						if !expectedCookieTime.Equal(ts) {
							t.Errorf(
								"Expected cookie create timestamp %v, got %v",
								expectedCookieTime,
								ts,
							)
						}
					}
				},
			)

			t.Run(
				"auth-token",
				func(t *testing.T) {
					token := e.AuthToken()
					if token != nil {
						t.Errorf("Expected nil auth token, got %+v", *token)
					}
				},
			)
		},
	)

	t.Run(
		"valid-auth",
		func(t *testing.T) {
			ctx := thrift.SetHeader(
				context.Background(),
				thriftbp.HeaderEdgeRequest,
				headerWithValidAuth,
			)
			e, err := edgecontext.FromThriftContext(ctx, globalTestImpl)
			if err != nil {
				t.Fatal(err)
			}

			if e.SessionID() != expectedSessionID {
				t.Errorf(
					"Expected session id %q, got %q",
					expectedSessionID,
					e.SessionID(),
				)
			}

			if e.DeviceID() != expectedDeviceID {
				t.Errorf(
					"Expected device id %q, got %q",
					expectedDeviceID,
					e.DeviceID(),
				)
			}

			t.Run(
				"user",
				func(t *testing.T) {
					user := e.User()
					if !user.IsLoggedIn() {
						t.Error("Expected logged in user, IsLoggedIn() returned false")
					}
					if user, ok := user.ID(); !ok {
						t.Error("Failed to get user id")
					} else {
						if user != expectedUser {
							t.Errorf("Expected user id %q, got %q", expectedUser, user)
						}
					}
					if loid, ok := user.LoID(); !ok {
						t.Error("Failed to get loid from user")
					} else {
						if loid != expectedLoID {
							t.Errorf("LoID expected %q, got %q", expectedLoID, loid)
						}
					}
					if ts, ok := user.CookieCreatedAt(); !ok {
						t.Error("Failed to get cookie created time from user")
					} else {
						if !expectedCookieTime.Equal(ts) {
							t.Errorf(
								"Expected cookie create timestamp %v, got %v",
								expectedCookieTime,
								ts,
							)
						}
					}
					if len(user.Roles()) != 0 {
						t.Errorf("Expected empty roles, got %+v", user.Roles())
					}
				},
			)
		},
	)

	t.Run(
		"expired-auth",
		func(t *testing.T) {
			ctx := thrift.SetHeader(
				context.Background(),
				thriftbp.HeaderEdgeRequest,
				headerWithExpiredAuth,
			)
			e, err := edgecontext.FromThriftContext(ctx, globalTestImpl)
			if err != nil {
				t.Fatal(err)
			}

			if e.SessionID() != expectedSessionID {
				t.Errorf(
					"Expected session id %q, got %q",
					expectedSessionID,
					e.SessionID(),
				)
			}

			t.Run(
				"user",
				func(t *testing.T) {
					user := e.User()
					if user.IsLoggedIn() {
						t.Error("Expected logged out user, IsLoggedIn() returned true")
					}
					if loid, ok := user.LoID(); !ok {
						t.Error("Failed to get loid from user")
					} else {
						if loid != expectedLoID {
							t.Errorf("LoID expected %q, got %q", expectedLoID, loid)
						}
					}
					if ts, ok := user.CookieCreatedAt(); !ok {
						t.Error("Failed to get cookie created time from user")
					} else {
						if !expectedCookieTime.Equal(ts) {
							t.Errorf(
								"Expected cookie create timestamp %v, got %v",
								expectedCookieTime,
								ts,
							)
						}
					}
				},
			)

			t.Run(
				"auth-token",
				func(t *testing.T) {
					token := e.AuthToken()
					if token != nil {
						t.Errorf("Expected nil auth token, got %+v", *token)
					}
				},
			)
		},
	)

	t.Run(
		"anon-auth",
		func(t *testing.T) {
			ctx := thrift.SetHeader(
				context.Background(),
				thriftbp.HeaderEdgeRequest,
				headerWithAnonAuth,
			)
			e, err := edgecontext.FromThriftContext(ctx, globalTestImpl)
			if err != nil {
				t.Fatal(err)
			}

			if e.SessionID() != expectedSessionID {
				t.Errorf(
					"Expected session id %q, got %q",
					expectedSessionID,
					e.SessionID(),
				)
			}

			t.Run(
				"user",
				func(t *testing.T) {
					user := e.User()
					if user.IsLoggedIn() {
						t.Error("Expected logged out user, IsLoggedIn() returned true")
					}
					if loid, ok := user.LoID(); !ok {
						t.Error("Failed to get loid from user")
					} else {
						if loid != expectedLoID {
							t.Errorf("LoID expected %q, got %q", expectedLoID, loid)
						}
					}
					if ts, ok := user.CookieCreatedAt(); !ok {
						t.Error("Failed to get cookie created time from user")
					} else {
						if !expectedCookieTime.Equal(ts) {
							t.Errorf(
								"Expected cookie create timestamp %v, got %v",
								expectedCookieTime,
								ts,
							)
						}
					}
					if !user.HasRole("anonymous") {
						t.Errorf(
							"Expected user to have anonymous role, got %+v",
							user.Roles(),
						)
					}
				},
			)
		},
	)
}

func TestFromHTTPContext(t *testing.T) {
	const expectedUser = "t2_example"

	t.Run(
		"no-header",
		func(t *testing.T) {
			ctx := context.Background()
			_, err := edgecontext.FromHTTPContext(ctx, globalTestImpl)
			if !errors.Is(err, edgecontext.ErrNoHeader) {
				t.Errorf("Expected ErrNoHeader, got %v", err)
			}
		},
	)

	t.Run(
		"no-auth",
		func(t *testing.T) {
			ctx := httpbp.SetHeader(
				context.Background(),
				httpbp.EdgeContextContextKey,
				headerWithNoAuth,
			)
			e, err := edgecontext.FromHTTPContext(ctx, globalTestImpl)
			if err != nil {
				t.Fatal(err)
			}

			if e.SessionID() != expectedSessionID {
				t.Errorf(
					"Expected session id %q, got %q",
					expectedSessionID,
					e.SessionID(),
				)
			}

			t.Run(
				"user",
				func(t *testing.T) {
					user := e.User()
					if user.IsLoggedIn() {
						t.Error("Expected logged out user, IsLoggedIn() returned true")
					}
					if loid, ok := user.LoID(); !ok {
						t.Error("Failed to get loid from user")
					} else {
						if loid != expectedLoID {
							t.Errorf("LoID expected %q, got %q", expectedLoID, loid)
						}
					}
					if ts, ok := user.CookieCreatedAt(); !ok {
						t.Error("Failed to get cookie created time from user")
					} else {
						if !expectedCookieTime.Equal(ts) {
							t.Errorf(
								"Expected cookie create timestamp %v, got %v",
								expectedCookieTime,
								ts,
							)
						}
					}
				},
			)

			t.Run(
				"auth-token",
				func(t *testing.T) {
					token := e.AuthToken()
					if token != nil {
						t.Errorf("Expected nil auth token, got %+v", *token)
					}
				},
			)
		},
	)

	t.Run(
		"valid-auth",
		func(t *testing.T) {
			ctx := httpbp.SetHeader(
				context.Background(),
				httpbp.EdgeContextContextKey,
				headerWithValidAuth,
			)
			e, err := edgecontext.FromHTTPContext(ctx, globalTestImpl)
			if err != nil {
				t.Fatal(err)
			}

			if e.SessionID() != expectedSessionID {
				t.Errorf(
					"Expected session id %q, got %q",
					expectedSessionID,
					e.SessionID(),
				)
			}

			t.Run(
				"user",
				func(t *testing.T) {
					user := e.User()
					if !user.IsLoggedIn() {
						t.Error("Expected logged in user, IsLoggedIn() returned false")
					}
					if user, ok := user.ID(); !ok {
						t.Error("Failed to get user id")
					} else {
						if user != expectedUser {
							t.Errorf("Expected user id %q, got %q", expectedUser, user)
						}
					}
					if loid, ok := user.LoID(); !ok {
						t.Error("Failed to get loid from user")
					} else {
						if loid != expectedLoID {
							t.Errorf("LoID expected %q, got %q", expectedLoID, loid)
						}
					}
					if ts, ok := user.CookieCreatedAt(); !ok {
						t.Error("Failed to get cookie created time from user")
					} else {
						if !expectedCookieTime.Equal(ts) {
							t.Errorf(
								"Expected cookie create timestamp %v, got %v",
								expectedCookieTime,
								ts,
							)
						}
					}
					if len(user.Roles()) != 0 {
						t.Errorf("Expected empty roles, got %+v", user.Roles())
					}
				},
			)
		},
	)

	t.Run(
		"expired-auth",
		func(t *testing.T) {
			ctx := httpbp.SetHeader(
				context.Background(),
				httpbp.EdgeContextContextKey,
				headerWithExpiredAuth,
			)
			e, err := edgecontext.FromHTTPContext(ctx, globalTestImpl)
			if err != nil {
				t.Fatal(err)
			}

			if e.SessionID() != expectedSessionID {
				t.Errorf(
					"Expected session id %q, got %q",
					expectedSessionID,
					e.SessionID(),
				)
			}

			t.Run(
				"user",
				func(t *testing.T) {
					user := e.User()
					if user.IsLoggedIn() {
						t.Error("Expected logged out user, IsLoggedIn() returned true")
					}
					if loid, ok := user.LoID(); !ok {
						t.Error("Failed to get loid from user")
					} else {
						if loid != expectedLoID {
							t.Errorf("LoID expected %q, got %q", expectedLoID, loid)
						}
					}
					if ts, ok := user.CookieCreatedAt(); !ok {
						t.Error("Failed to get cookie created time from user")
					} else {
						if !expectedCookieTime.Equal(ts) {
							t.Errorf(
								"Expected cookie create timestamp %v, got %v",
								expectedCookieTime,
								ts,
							)
						}
					}
				},
			)

			t.Run(
				"auth-token",
				func(t *testing.T) {
					token := e.AuthToken()
					if token != nil {
						t.Errorf("Expected nil auth token, got %+v", *token)
					}
				},
			)
		},
	)

	t.Run(
		"anon-auth",
		func(t *testing.T) {
			ctx := httpbp.SetHeader(
				context.Background(),
				httpbp.EdgeContextContextKey,
				headerWithAnonAuth,
			)
			e, err := edgecontext.FromHTTPContext(ctx, globalTestImpl)
			if err != nil {
				t.Fatal(err)
			}

			if e.SessionID() != expectedSessionID {
				t.Errorf(
					"Expected session id %q, got %q",
					expectedSessionID,
					e.SessionID(),
				)
			}

			t.Run(
				"user",
				func(t *testing.T) {
					user := e.User()
					if user.IsLoggedIn() {
						t.Error("Expected logged out user, IsLoggedIn() returned true")
					}
					if loid, ok := user.LoID(); !ok {
						t.Error("Failed to get loid from user")
					} else {
						if loid != expectedLoID {
							t.Errorf("LoID expected %q, got %q", expectedLoID, loid)
						}
					}
					if ts, ok := user.CookieCreatedAt(); !ok {
						t.Error("Failed to get cookie created time from user")
					} else {
						if !expectedCookieTime.Equal(ts) {
							t.Errorf(
								"Expected cookie create timestamp %v, got %v",
								expectedCookieTime,
								ts,
							)
						}
					}
					if !user.HasRole("anonymous") {
						t.Errorf(
							"Expected user to have anonymous role, got %+v",
							user.Roles(),
						)
					}
				},
			)
		},
	)
}
