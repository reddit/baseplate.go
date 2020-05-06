package httpbp_test

import (
	"bytes"
	"encoding/json"
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
			if err := cw.WriteBody(&buf, map[string]int{"x": x}); err != nil {
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
			if err := cw.WriteBody(&buf, expected); err != nil {
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

	cw := httpbp.HTMLContentWriter(tmpl)
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
				err := cw.WriteBody(&buf, c.resp)
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

type testStringer struct {
	content string
}

func (s testStringer) String() string {
	return s.content
}

func TestRawContentWriterFactory(t *testing.T) {
	t.Parallel()

	content := "test"

	cw := httpbp.RawContentWriter(httpbp.PlainTextContentType)
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
			name:     "fmt.Stringer",
			body:     testStringer{content: content},
			expected: expectation{body: content},
		},
		{
			name:     "nil",
			body:     nil,
			expected: expectation{body: ""},
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
				err := cw.WriteBody(&buf, c.body)
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

func TestResponseWithCode(t *testing.T) {
	t.Parallel()

	r := httpbp.NewResponse("test")
	if r.Code != 0 {
		t.Errorf("wrong code, expected %d, got %d", 0, r.Code)
	}

	r = r.WithCode(http.StatusAccepted)
	if r.Code != http.StatusAccepted {
		t.Errorf("wrong code, expected %d, got %d", http.StatusAccepted, r.Code)
	}
}
