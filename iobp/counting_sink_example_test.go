package iobp_test

import (
	"encoding/json"
	"fmt"

	"github.com/reddit/baseplate.go/iobp"
)

// This example demonstrates how to use CountingSink to count the size of a
// json-serialized object.
func ExampleCountingSink() {
	// The json string would be "[0,1,2,3,4]\n", so the size should be 12.
	object := []int{0, 1, 2, 3, 4}

	var sink iobp.CountingSink
	if err := json.NewEncoder(&sink).Encode(object); err != nil {
		panic(err)
	}
	fmt.Printf("JSON size for %#v: %d\n", object, sink.Size())

	// Output:
	// JSON size for []int{0, 1, 2, 3, 4}: 12
}
