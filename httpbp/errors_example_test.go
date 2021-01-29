package httpbp_test

import (
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/log"
)

// This example demonstrates how you can read the response body and add that
// back to ClientError's AdditionalInfo field.
func ExampleClientError() {
	resp, err := http.Get("https://www.reddit.com")
	if err != nil {
		log.Errorw("http request failed", "err", err)
		return
	}
	defer httpbp.DrainAndClose(resp.Body)
	err = httpbp.ClientErrorFromResponse(resp)
	if err != nil {
		var ce *httpbp.ClientError
		if errors.As(err, &ce) {
			if body, e := ioutil.ReadAll(resp.Body); e == nil {
				ce.AdditionalInfo = string(body)
			}
		}
		log.Errorw("http request failed", "err", err)
		return
	}
	// now read/handle body normally
}

// This example demonstrates how DrainAndClose should be used with http
// requests.
func ExampleDrainAndClose() {
	resp, err := http.Get("https://www.reddit.com")
	if err != nil {
		log.Errorw("http request failed", "err", err)
		return
	}
	defer httpbp.DrainAndClose(resp.Body)
	// now read/handle body normally
}
