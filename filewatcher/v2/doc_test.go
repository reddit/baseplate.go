package filewatcher_test

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/reddit/baseplate.go/filewatcher/v2"
	"github.com/reddit/baseplate.go/log"
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
	type dataType map[string]any

	// Wrap a json decoder as parser
	parser := func(f io.Reader) (dataType, error) {
		var data dataType
		err := json.NewDecoder(f).Decode(&data)
		return data, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	data, err := filewatcher.New(ctx, path, parser)
	if err != nil {
		log.Fatal(err)
	}

	// Whenever you need to use the parsed data, just call data.Get():
	_ = data.Get()
}
