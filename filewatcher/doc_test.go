package filewatcher_test

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"time"

	"github.snooguts.net/reddit/baseplate.go/filewatcher"
)

// This example demonstrates how to use filewatcher.
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
	data, err := filewatcher.New(
		ctx,
		path,
		parser,
		nil, // logger
	)
	if err != nil {
		log.Panic(err)
	}

	getData := func() dataType {
		return data.Get().(dataType)
	}

	// Whenever you need to use the parsed data, just call getData():
	_ = getData()
}

// This example demonstrates how to use filewatcher with a global variable for
// parsed data maintained by the caller.
func Example_globalParsedData() {
	const (
		// The path to the file.
		path = "/opt/data.json"
		// Timeout on the initial read.
		timeout = time.Second * 30
	)

	// The type of the parsed data
	type dataType map[string]interface{}
	var parsed dataType

	// Wrap a json decoder as parser.
	// Note that instead of returning the parsed data,
	// we assign it to parsed instead.
	parser := func(f io.Reader) (interface{}, error) {
		var data dataType
		err := json.NewDecoder(f).Decode(&data)
		if err != nil {
			parsed = data
		}
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := filewatcher.New(
		ctx,
		path,
		parser,
		nil, // logger
	)
	if err != nil {
		log.Panic(err)
	}
	// parsed is ready to use.
	_ = parsed
}
