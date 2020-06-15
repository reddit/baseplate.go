package filter

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

type helloFilter struct {
	msg string
}

func (f *helloFilter) Do(request interface{}, service Service) (rsp interface{}, err error) {
	rsp, err = service.Do(request)
	httpRsp, ok := rsp.(*http.Response)
	if ok {
		if err == nil {
			httpRsp.Header.Add("hello", f.msg)
		}
	} else {
		err = errors.New("not an http response")
	}
	return
}

type slowFilter struct {
	duration time.Duration
}

func (f *slowFilter) Do(request interface{}, service Service) (rsp interface{}, err error) {
	time.Sleep(f.duration)
	return service.Do(request)
}

func TestHttpClientWithSpecificFilter(t *testing.T) {
	client := HTTPClientWithFilters(&http.Client{}, &helloFilter{msg: "world"})
	rsp, _ := client.Get("https://google.com/")
	if rsp.Header.Get("hello") != "world" {
		t.Errorf("didn't set response header")
	}
}
func TestHttpClientWithGenericFilter(t *testing.T) {
	sleepFor := 1 * time.Second
	client := HTTPClientWithFilters(&http.Client{}, &slowFilter{duration: sleepFor})
	start := time.Now()
	_, _ = client.Get("https://google.com/")
	if time.Now().Sub(start) < sleepFor {
		t.Error("Didn't sleep long enough")
	}
}
