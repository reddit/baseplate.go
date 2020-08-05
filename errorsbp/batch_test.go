package errorsbp_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/reddit/baseplate.go/errorsbp"
)

func TestAdd(t *testing.T) {
	var err errorsbp.Batch
	if len(err.GetErrors()) != 0 {
		t.Errorf("A new BatchError should contain zero errors: %#v", err.GetErrors())
	}

	err.Add(nil)
	if len(err.GetErrors()) != 0 {
		t.Errorf("Nil errors should be skipped: %#v", err.GetErrors())
	}

	err0 := errors.New("foo")
	err.Add(err0)
	if len(err.GetErrors()) != 1 {
		t.Errorf("Non-nil errors should be added to the batch: %#v", err.GetErrors())
	}
	actual := err.GetErrors()[0]
	if actual != err0 {
		t.Errorf("Expected %v, got %v", err0, actual)
	}

	var another errorsbp.Batch
	err.Add(another)
	if len(err.GetErrors()) != 1 {
		t.Errorf("Empty batch should be skipped: %#v", err.GetErrors())
	}
	err1 := errors.New("bar")
	another.Add(err1)
	err2 := errors.New("foobar")
	another.Add(err2)
	err.Add(another)
	if len(err.GetErrors()) != 3 {
		t.Errorf(
			"The underlying errors should be added instead of the batch: %#v",
			err.GetErrors(),
		)
	}

	batch := err.GetErrors()
	if batch[0] != err0 {
		t.Errorf("Expected %v, got %v", err0, batch[0])
	}
	if batch[1] != err1 {
		t.Errorf("Expected %v, got %v", err1, batch[1])
	}
	if batch[2] != err2 {
		t.Errorf("Expected %v, got %v", err2, batch[2])
	}

	err.Clear()
	if len(err.GetErrors()) != 0 {
		t.Errorf(
			"A cleared BatchError should contain zero errors: %#v",
			err.GetErrors(),
		)
	}

	pointer := new(errorsbp.Batch)
	err.Add(pointer)
	if len(err.GetErrors()) != 0 {
		t.Errorf("Empty batch should be skipped: %#v", err.GetErrors())
	}
	err1 = errors.New("bar")
	pointer.Add(err1)
	err2 = errors.New("foobar")
	pointer.Add(err2)
	err.Add(pointer)
	if len(err.GetErrors()) != 2 {
		t.Errorf(
			"The underlying errors should be added instead of the batch: %#v",
			err.GetErrors(),
		)
	}
}

func TestAddMultiple(t *testing.T) {
	var err errorsbp.Batch
	if len(err.GetErrors()) != 0 {
		t.Errorf("A new BatchError should contain zero errors: %#v", err.GetErrors())
	}

	err.Add()
	if len(err.GetErrors()) != 0 {
		t.Errorf("empty errors should be skipped: %#v", err.GetErrors())
	}

	err.Add(nil, nil)
	if len(err.GetErrors()) != 0 {
		t.Errorf("empty errors should be skipped: %#v", err.GetErrors())
	}

	err0 := errors.New("foo")
	err1 := errors.New("bar")
	err.Add(err0, err1)
	if len(err.GetErrors()) != 2 {
		t.Errorf("Non-nil errors should be added to the batch: %v", err.GetErrors())
	}
	actual0 := err.GetErrors()[0]
	if actual0 != err0 {
		t.Errorf("Expected %v, got %v", err0, actual0)
	}
	actual1 := err.GetErrors()[1]
	if actual1 != err1 {
		t.Errorf("Expected %v, got %v", err1, actual1)
	}

	var another errorsbp.Batch
	err.Add(another)
	if len(err.GetErrors()) != 2 {
		t.Errorf("Empty batch should be skipped: %#v", err.GetErrors())
	}
	err2 := errors.New("fizz")
	err3 := errors.New("buzz")
	err4 := errors.New("alpha")
	another.Add(err2, err3)
	err.Add(another, err4)
	if len(err.GetErrors()) != 5 {
		t.Errorf(
			"The underlying errors should be added instead of the batch: %#v",
			err.GetErrors(),
		)
	}

	batch := err.GetErrors()
	if batch[0] != err0 {
		t.Errorf("Expected %v, got %v", err0, batch[0])
	}
	if batch[1] != err1 {
		t.Errorf("Expected %v, got %v", err1, batch[1])
	}
	if batch[2] != err2 {
		t.Errorf("Expected %v, got %v", err2, batch[2])
	}
	if batch[3] != err3 {
		t.Errorf("Expected %v, got %v", err3, batch[3])
	}
	if batch[4] != err4 {
		t.Errorf("Expected %v, got %v", err4, batch[4])
	}
}

func TestCompile(t *testing.T) {
	var batch errorsbp.Batch
	err0 := errors.New("foo")
	err1 := errors.New("bar")
	err2 := errors.New("foobar")

	err := batch.Compile()
	if err != nil {
		t.Errorf("An empty batch should be compiled to nil, got: %v", err)
	}
	batch.Add(err0)
	err = batch.Compile()
	if err != err0 {
		t.Errorf(
			"A single error batch should be compiled to %v, got %v",
			err0,
			err,
		)
	}
	batch.Add(err1)
	batch.Add(err2)
	err = batch.Compile()
	expect := "errorsbp.Batch: total 3 error(s) in this batch: foo; bar; foobar"
	if err.Error() != expect {
		t.Errorf("Compiled error expected %q, got %v", expect, err)
	}

	errString := batch.Error()
	if errString != expect {
		t.Errorf("Compiled error expected %q, got %q", expect, errString)
	}
}

func TestGetErrors(t *testing.T) {
	var batch errorsbp.Batch
	err0 := errors.New("foo")
	err1 := errors.New("bar")
	err2 := errors.New("foobar")

	batch.Add(err0)
	batch.Add(err1)
	batch.Add(err2)
	errs := batch.GetErrors()
	expect := []error{err0, err1, err2}
	if !reflect.DeepEqual(errs, expect) {
		t.Errorf("GetErrors expected %#v, got %#v", expect, errs)
	}

	errs[2] = err1
	errs = batch.GetErrors()
	if !reflect.DeepEqual(errs, expect) {
		t.Errorf(
			"GetErrors should return a copy, not the original slice. Expected %#v, got %#v",
			expect,
			errs,
		)
	}
}

func TestAddPrefix(t *testing.T) {
	const (
		prefix1 = "prefix1"
		prefix2 = "pre%sfix2"

		separator = ": "

		msg0 = "foo"
		msg1 = "bar"
		msg2 = "foobar"
	)
	var batch errorsbp.Batch
	err0 := errors.New(msg0)
	err1 := errors.New(msg1)
	err2 := errors.New(msg2)

	batch.AddPrefix(prefix1, nil)
	if len(batch.GetErrors()) != 0 {
		t.Errorf("Nil errors should be skipped: %#v", batch.GetErrors())
	}

	batch.AddPrefix(prefix1, err0)
	if len(batch.GetErrors()) != 1 {
		t.Errorf("Non-nil errors should be added to the batch: %#v", batch.GetErrors())
	}

	if !errors.Is(batch, err0) {
		t.Errorf("Expected batch to contain %v, got false: %#v", err0, batch.GetErrors())
	}

	var batch2 errorsbp.Batch
	batch2.AddPrefix(prefix2, err1)
	batch2.AddPrefix("", err2)

	batch.AddPrefix(prefix1, batch2)
	errs := batch.GetErrors()
	if !errors.Is(batch, err1) {
		t.Errorf("Expected batch to contain %v, got false: %#v", err1, errs)
	}
	if !errors.Is(batch, err2) {
		t.Errorf("Expected batch to contain %v, got false: %#v", err2, errs)
	}
	if len(errs) != 3 {
		t.Fatalf("Expected Batch to be flattened while using AddPrefix, got: %#v", errs)
	}

	expectedMsgs := []string{
		prefix1 + separator + msg0,
		prefix1 + separator + prefix2 + separator + msg1,
		prefix1 + separator + msg2,
	}
	for i, err := range errs {
		if err.Error() != expectedMsgs[i] {
			t.Errorf("Expected %dth error to be %q, got %v", i, expectedMsgs[i], err)
		}
	}
}
