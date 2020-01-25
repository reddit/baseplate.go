package batcherror_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/reddit/baseplate.go/batcherror"
)

func TestAdd(t *testing.T) {
	var err batcherror.BatchError
	if len(err.GetErrors()) != 0 {
		t.Errorf("A new BatchError should contain zero errors: %v", err.GetErrors())
	}

	err.Add(nil)
	if len(err.GetErrors()) != 0 {
		t.Errorf("Nil errors should be skipped: %v", err.GetErrors())
	}

	err0 := errors.New("foo")
	err.Add(err0)
	if len(err.GetErrors()) != 1 {
		t.Errorf("Non-nil errors should be added to the batch: %v", err.GetErrors())
	}
	actual := err.GetErrors()[0]
	if actual != err0 {
		t.Errorf("Expected %#v, got %#v", err0, actual)
	}

	var another batcherror.BatchError
	err.Add(another)
	if len(err.GetErrors()) != 1 {
		t.Errorf("Empty batch should be skipped: %v", err.GetErrors())
	}
	err1 := errors.New("bar")
	another.Add(err1)
	err2 := errors.New("foobar")
	another.Add(err2)
	err.Add(another)
	if len(err.GetErrors()) != 3 {
		t.Errorf(
			"The underlying errors should be added instead of the batch: %v",
			err.GetErrors(),
		)
	}

	batch := err.GetErrors()
	if batch[0] != err0 {
		t.Errorf("Expected %#v, got %#v", err0, batch[0])
	}
	if batch[1] != err1 {
		t.Errorf("Expected %#v, got %#v", err1, batch[1])
	}
	if batch[2] != err2 {
		t.Errorf("Expected %#v, got %#v", err2, batch[2])
	}

	err.Clear()
	if len(err.GetErrors()) != 0 {
		t.Errorf(
			"A cleared BatchError should contain zero errors: %v",
			err.GetErrors(),
		)
	}

	pointer := new(batcherror.BatchError)
	err.Add(pointer)
	if len(err.GetErrors()) != 0 {
		t.Errorf("Empty batch should be skipped: %v", err.GetErrors())
	}
	err1 = errors.New("bar")
	pointer.Add(err1)
	err2 = errors.New("foobar")
	pointer.Add(err2)
	err.Add(pointer)
	if len(err.GetErrors()) != 2 {
		t.Errorf(
			"The underlying errors should be added instead of the batch: %v",
			err.GetErrors(),
		)
	}
}

func TestCompile(t *testing.T) {
	var batch batcherror.BatchError
	err0 := errors.New("foo")
	err1 := errors.New("bar")
	err2 := errors.New("foobar")

	err := batch.Compile()
	if err != nil {
		t.Errorf("An empty batch should be compiled to nil, got: %#v", err)
	}
	batch.Add(err0)
	err = batch.Compile()
	if err != err0 {
		t.Errorf(
			"A single error batch should be compiled to %#v, got %#v",
			err0,
			err,
		)
	}
	batch.Add(err1)
	batch.Add(err2)
	err = batch.Compile()
	expect := "batcherror: total 3 error(s) in this batch: foo; bar; foobar"
	if err.Error() != expect {
		t.Errorf("Compiled error expected %#v, got %#v", expect, err)
	}

	errString := batch.Error()
	if errString != expect {
		t.Errorf("Compiled error expected %#v, got %#v", expect, errString)
	}
}

func TestGetErrors(t *testing.T) {
	var batch batcherror.BatchError
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
