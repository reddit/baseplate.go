package httpbp_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/reddit/baseplate.go/httpbp"
)

type htmlResponseBody struct {
	httpbp.BaseHTMLBody

	Y int
}

const (
	htmlTemplateOK  = `<b>OK: {{.Y}}</b>`
	htmlTemplateErr = `<b>ERROR: {{.Y}}</b>`

	x = 1
	y = 2
)

func TestJSONContentWriter(t *testing.T) {
	t.Parallel()

	cw := httpbp.JSONContentWriter()
	if cw.ContentType() != httpbp.JSONContentType {
		t.Errorf("wrong content-type %q", cw.ContentType())
	}

	t.Run(
		"map",
		func(t *testing.T) {
			t.Parallel()

			expected := map[string]int{"x": x}

			var buf bytes.Buffer
			if err := cw.WriteResponse(&buf, map[string]int{"x": x}); err != nil {
				t.Fatal(err)
			}

			var got map[string]int
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(expected, got) {
				t.Errorf("map mismatch, expected %#v, got %#v", expected, got)
			}
		},
	)

	t.Run(
		"struct",
		func(t *testing.T) {
			t.Parallel()

			expected := jsonResponseBody{X: x}

			var buf bytes.Buffer
			if err := cw.WriteResponse(&buf, expected); err != nil {
				t.Fatal(err)
			}

			var got jsonResponseBody
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(expected, got) {
				t.Errorf("map mismatch, expected %#v, got %#v", expected, got)
			}
		},
	)
}

func TestHTMLContentWriterFactory(t *testing.T) {
	t.Parallel()

	const (
		okName  = "ok"
		errName = "err"
	)

	tmpl, err := template.New(okName).Parse(htmlTemplateOK)
	if err != nil {
		t.Fatal(err)
	}

	tmpl, err = tmpl.New(errName).Parse(htmlTemplateErr)
	if err != nil {
		t.Fatal(err)
	}

	cw := httpbp.HTMLContentWriterFactory(tmpl)()
	if cw.ContentType() != httpbp.HTMLContentType {
		t.Errorf("wrong content-type %q", cw.ContentType())
	}

	type expectation struct {
		body string
		err  bool
	}
	cases := []struct {
		name     string
		resp     interface{}
		expected expectation
	}{
		{
			name: okName,
			resp: htmlResponseBody{
				BaseHTMLBody: httpbp.BaseHTMLBody{Name: okName},
				Y:            y,
			},
			expected: expectation{body: fmt.Sprintf("<b>OK: %d</b>", y)},
		},
		{
			name: errName,
			resp: htmlResponseBody{
				BaseHTMLBody: httpbp.BaseHTMLBody{Name: errName},
				Y:            y,
			},
			expected: expectation{body: fmt.Sprintf("<b>ERROR: %d</b>", y)},
		},
		{
			name: "template-missing",
			resp: htmlResponseBody{
				BaseHTMLBody: httpbp.BaseHTMLBody{Name: "missinsg"},
				Y:            y,
			},
			expected: expectation{err: true},
		},
		{
			name:     "wrong-type",
			resp:     jsonResponseBody{X: x},
			expected: expectation{err: true},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()

				var buf bytes.Buffer
				err := cw.WriteResponse(&buf, c.resp)
				if c.expected.err && err == nil {
					t.Errorf("expected error, got nil")
				} else if !c.expected.err && err != nil {
					t.Fatal(err)
				}

				if buf.String() != c.expected.body {
					t.Errorf(
						"body mismatch, expected %q, got %q",
						c.expected.body,
						buf.String(),
					)
				}
			},
		)
	}
}

func TestRawContentWriterFactory(t *testing.T) {
	t.Parallel()

	content := "test"

	cw := httpbp.RawContentWriterFactory(httpbp.PlainTextContentType)()
	if cw.ContentType() != httpbp.PlainTextContentType {
		t.Errorf("wrong content-type %q", cw.ContentType())
	}

	type expectation struct {
		body string
		err  bool
	}

	cases := []struct {
		name     string
		body     interface{}
		expected expectation
	}{
		{
			name:     "string",
			body:     content,
			expected: expectation{body: content},
		},
		{
			name:     "[]byte",
			body:     []byte(content),
			expected: expectation{body: content},
		},
		{
			name:     "io.Reader",
			body:     strings.NewReader(content),
			expected: expectation{body: content},
		},
		{
			name:     "wrong-type",
			body:     1,
			expected: expectation{err: true},
		},
	}
	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				t.Parallel()

				var buf bytes.Buffer
				err := cw.WriteResponse(&buf, c.body)
				if c.expected.err && err == nil {
					t.Errorf("expected error, got nil")
				} else if !c.expected.err && err != nil {
					t.Fatal(err)
				}

				if buf.String() != c.expected.body {
					t.Errorf("body mismatch, expected %q, got %q", c.expected.body, buf.String())
				}
			},
		)
	}
}

func TestResponse(t *testing.T) {
	t.Parallel()

	resp := httpbp.NewResponse(httpbp.JSONContentWriter)

	t.Run(
		"StatusCode",
		func(t *testing.T) {
			if resp.StatusCode() != 0 {
				t.Errorf("wrong default status code, %d", resp.StatusCode())
			}

			resp.SetCode(http.StatusPermanentRedirect)
			if resp.StatusCode() != http.StatusPermanentRedirect {
				t.Errorf(
					"wrong status code, expected %d, got %d",
					http.StatusPermanentRedirect,
					resp.StatusCode(),
				)
			}
		},
	)

	t.Run(
		"Headers",
		func(t *testing.T) {
			if len(resp.Headers()) != 0 {
				t.Fatalf("wrong default headers %#v", resp.Headers())
			}

			resp.Headers().Set("foo", "bar")
			if len(resp.Headers()) != 1 {
				t.Fatalf("wrong headers %#v", resp.Headers())
			}

			resp.Headers().Del("foo")
			if len(resp.Headers()) != 0 {
				t.Fatalf("key not deleted %#v", resp.Headers())
			}
		},
	)

	t.Run(
		"Cookies",
		func(t *testing.T) {
			if len(resp.Cookies()) != 0 {
				t.Fatalf("wrong default cookies %#v", resp.Cookies())
			}

			resp.AddCookie(&http.Cookie{})
			if len(resp.Cookies()) != 1 {
				t.Fatalf("wrong cookies %#v", resp.Cookies())
			}

			resp.ClearCookies()
			if len(resp.Cookies()) != 0 {
				t.Fatalf("cookies not cleared %#v", resp.Cookies())
			}
		},
	)

	t.Run("ContentWriter", func(t *testing.T) {
		cw := resp.ContentWriter()
		if cw.ContentType() != httpbp.JSONContentType {
			t.Errorf("wrong content-type %q", cw.ContentType())
		}

		resp.SetContentWriter(httpbp.HTMLContentWriterFactory(nil)())
		defer func() {
			resp.SetContentWriter(cw)
		}()

		if resp.ContentWriter().ContentType() != httpbp.HTMLContentType {
			t.Errorf("wrong content-type %q", resp.ContentWriter().ContentType())
		}
	})

	t.Run(
		"NewHTTPError",
		func(t *testing.T) {
			// Add a header and cookie to test that they do not propogate to
			// the error.
			resp.Headers().Set("foo", "bar")
			resp.AddCookie(&http.Cookie{})
			defer func() {
				resp.ClearCookies()
				resp.Headers().Del("foo")
			}()

			err := resp.NewHTTPError(
				http.StatusInternalServerError,
				jsonResponseBody{X: x},
				errors.New("test"),
			)

			if len(err.Headers()) != 0 {
				t.Fatalf("headers not empty %#v", err.Headers())
			}
			if len(err.Cookies()) != 0 {
				t.Fatalf("cookies not empty %#v", err.Cookies())
			}

			cw := err.ContentWriter()
			if cw.ContentType() != httpbp.JSONContentType {
				t.Errorf("wrong content-type %q", cw.ContentType())
			}

			if err.StatusCode() != http.StatusInternalServerError {
				t.Errorf(
					"wrong status code, expected %d, got %d",
					http.StatusInternalServerError,
					err.StatusCode(),
				)
			}
		},
	)
}
