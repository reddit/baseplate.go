package dirwatcher_test

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/reddit/baseplate.go/dirwatcher"
	"github.com/reddit/baseplate.go/internal/limitopen"
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
		reader := f.(limitopen.ReadCloser)
		var data dataType
		folder := make(map[string]interface{})
		err := json.NewDecoder(f).Decode(&data)

		folder[reader.Path] = data
		return folder, err
	}

	// a function that knows how to add a files data from the interface
	adder := func(d interface{}, file interface{}) (interface{}, error) {
		folder := d.(map[string]interface{})

		for key, value := range file.(map[string]interface{}) {
			folder[key] = value
		}
		return folder, nil

	}

	// a function that can clean up the data based on path name
	remover := func(d interface{}, path string) (interface{}, error) {
		folder := d.(map[string]interface{})
		delete(folder, path)
		return folder, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	data, err := dirwatcher.New(
		ctx,
		dirwatcher.Config{
			Path:    path,
			Parser:  parser,
			Adder:   adder,
			Remover: remover,
			Logger:  log.ErrorWithSentryWrapper(),
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
