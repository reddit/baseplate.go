package filter

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

//HTTPClientWithFilters applies filter middleware to an http client.
func HTTPClientWithFilters(client *http.Client, filters ...Filter) *Client {
	svc := HTTPClientAsService(client)
	withFilters := ServiceWithFilters(svc, filters...)
	return httpClientAdapter(withFilters)
}

func httpClientAdapter(service Service) *Client {
	return &Client{inner: service}
}

// HTTPClientAsService represents an http.Client as a Service
func HTTPClientAsService(client *http.Client) Service {
	return &httpClientService{client}
}

type httpClientService struct {
	client *http.Client
}

// Client is duck-typed like http.Client, but internally implemented by a Service.
type Client struct {
	inner Service
}

func (svc *httpClientService) Do(request interface{}) (response interface{}, err error) {
	httpRequest, ok := request.(*http.Request)
	if !ok {
		return nil, errors.New("not an http request")
	}
	return svc.client.Do(httpRequest)
}

// Do is a copy of http.Do
func (c *Client) Do(req *http.Request) (resp *http.Response, err error) {
	r, err := c.inner.Do(req)
	resp, ok := r.(*http.Response)
	if !ok && err == nil {
		return nil, errors.New("not an http response")
	}
	return
}

// Get is a copy of http.Get
func (c *Client) Get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Head is a copy of http.Head
func (c *Client) Head(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Post is a copy of http.Post
func (c *Client) Post(url, contentType string, body io.Reader) (resp *http.Response, err error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(req)
}

// PostForm is a copy of http.PostForm
func (c *Client) PostForm(url string, data url.Values) (resp *http.Response, err error) {
	return c.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}
