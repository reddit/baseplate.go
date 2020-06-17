package errorsbp_test

import (
	"errors"

	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
)

// BEGIN THRIFT GENERATED CODE
//
// In real code this part should come from thrift compiled go code.
// Here we just write some placeholders to use as an example.

type MyThriftException struct{}

func (*MyThriftException) Error() string {
	return "my thrift exception"
}

// END THRIFT GENERATED CODE

func MyThriftExceptionSuppressor(err error) bool {
	return errors.As(err, new(*MyThriftException))
}

func BaseplateErrorSuppressor(err error) bool {
	// baseplate.Error is from a thrift exception defined in baspleate.thrift,
	// then compiled to go code by thrift compiler.
	// We use that type as an example here.
	// In real code you usually also should add (Or) additional exceptions defined
	// in your thrift files.
	return errors.As(err, new(*baseplate.Error))
}

// This example demonstrates how to implement a Suppressor.
func ExampleSuppressor() {
	// This constructs the Suppressor you could fill into
	// thriftbp.ServerConfig.ErrorSpanSuppressor field.
	errorsbp.OrSuppressors(BaseplateErrorSuppressor, MyThriftExceptionSuppressor)
}
