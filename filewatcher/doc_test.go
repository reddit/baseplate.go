package filewatcher_test

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"time"

	"github.com/reddit/baseplate.go/filewatcher"
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
