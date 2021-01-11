package edgecontext_test

import (
	"testing"
)

// copied from https://github.com/reddit/edgecontext.py/blob/420e58728ee7085a2f91c5db45df233142b251f9/tests/edge_context_tests.py#L54
const validToken = `eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0Ml9leGFtcGxlIiwiZXhwIjoyNTI0NjA4MDAwfQ.dRzzfc9GmzyqfAbl6n_C55JJueraXk9pp3v0UYXw0ic6W_9RVa7aA1zJWm7slX9lbuYldwUtHvqaSsOpjF34uqr0-yMoRDVpIrbkwwJkNuAE8kbXGYFmXf3Ip25wMHtSXn64y2gJN8TtgAAnzjjGs9yzK9BhHILCDZTtmPbsUepxKmWTiEX2BdurUMZzinbcvcKY4Rb_Fl0pwsmBJFs7nmk5PvTyC6qivCd8ZmMc7dwL47mwy_7ouqdqKyUEdLoTEQ_psuy9REw57PRe00XCHaTSTRDCLmy4gAN6J0J056XoRHLfFcNbtzAmqmtJ_D9HGIIXPKq-KaggwK9I4qLX7g`

func TestValidToken(t *testing.T) {
	token, err := globalTestImpl.ValidateToken(validToken)
	if err != nil {
		t.Fatal(err)
	}
	expected := "t2_example"
	actual := token.Subject()
	if actual != expected {
		t.Errorf("subject expected %q, got %q", expected, actual)
	}
}
