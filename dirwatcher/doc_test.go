package dirwatcher_test

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/reddit/baseplate.go/dirwatcher"
	"github.com/reddit/baseplate.go/log"
)

// This example demonstrates how to use dirwatcher.
func Example() {
	const (
		// The path to the file.
		path = "/opt/data.json"
		// Timeout on the initial read.
		timeout = time.Second * 30
	)

	// The type of the parsed data
	type dataType map[string]interface{}

	// Wrap a json decoder as parser
	parser := func(f io.Reader) (interface{}, error) {
		var data dataType
		err := json.NewDecoder(f).Decode(&data)
		return data, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	data, err := dirwatcher.New(
		ctx,
		dirwatcher.Config{
			Path:   path,
			Parser: parser,
			Logger: log.ErrorWithSentryWrapper(),
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	getData := func() dataType {
		return data.Get().(dataType)
	}

	// Whenever you need to use the parsed data, just call getData():
	_ = getData()
}
