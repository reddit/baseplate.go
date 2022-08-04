package metricsbp

import (
	"reflect"
)

// CheckNilFields returns all the nil value fields inside root.
//
// root should be a value to a struct, or a pointer to a struct, otherwise this
// function will panic. The return value would be the field names of all the
// uninitialized fields recursively.
//
// For example, for the following code:
//
//	type Bar struct {
//	  A io.Reader
//	  B io.Reader
//	  c io.Reader
//	  D struct{
//	    A io.Reader
//	    B io.Reader
//	  }
//	}
//
//	func main() {
//	  fields := CheckNilInterfaceFields(
//	    &Bar{
//	      A: strings.NewReader("foo"),
//	      D: {
//	        B: bytes.NewReader([]bytes("bar")),
//	      },
//	    },
//	  )
//	}
//
// fields should contain 3 strings: "Bar.B", "Bar.c", and "Bar.D.A".
//
// Special case: When root itself is nil, or pointer to nil pointer, a single,
// empty string ("") will be returned.
//
// The main use case of this function is for pre-created metrics.
// A common pattern is to define a struct contains all the metrics,
// initialize them in main function,
// then pass the struct down to the handler functions to use.
// It has better performance over creating metrics on the fly when using,
// but comes with a down side that if you added a new metric to the struct but
// forgot to initialize it in the main function,
// the code would panic when it's first used.
//
// Use this function to check the metrics struct after initialization could help
// you panic earlier and in a more predictable way.
func CheckNilFields(root interface{}) []string {
	v := reflect.ValueOf(root)
	prefix := ""
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return []string{""}
		}
		prefix += "*"
		v = reflect.Indirect(v)
	}
	prefix += v.Type().Name()
	return checkNilFieldsRecursion(v, prefix)
}

func checkNilFieldsRecursion(v reflect.Value, prefix string) (fields []string) {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return []string{prefix}
		}
		v = reflect.Indirect(v)
	}

	prefix = prefix + "."
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		name := prefix + t.Field(i).Name
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.Interface:
			if fv.IsNil() {
				fields = append(fields, name)
			}
		case reflect.Ptr, reflect.Struct:
			fields = append(
				fields,
				checkNilFieldsRecursion(fv, name)...,
			)
		}
	}
	return
}
