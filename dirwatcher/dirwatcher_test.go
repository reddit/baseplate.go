package dirwatcher_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/dirwatcher"
	"github.com/reddit/baseplate.go/filewatcher"
	"github.com/reddit/baseplate.go/internal/limitopen"
	"github.com/reddit/baseplate.go/log"
)

func TestDirWatcher(t *testing.T) {
	dir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	data, err := dirwatcher.New(
		ctx,
		dirwatcher.Config{
			Path: dir,
			Parser: func(f io.Reader) (interface{}, error) {
				reader := f.(limitopen.ReadCloser)
				var data map[string]interface{}
				folder := make(map[string]interface{})
				err := json.NewDecoder(f).Decode(&data)
				if err != nil {
					t.Fatal(err)
				}

				folder[reader.Path] = data
				return folder, err
			},

			// a function that knows how to add a files data from the interface
			Adder: func(d interface{}, file interface{}) (interface{}, error) {
				if d == nil {
					d = make(map[string]interface{})
				}
				folder := d.(map[string]interface{})

				for key, value := range file.(map[string]interface{}) {
					folder[key] = value
				}
				return folder, nil

			},

			// a function that can clean up the data based on path name
			Remover: func(d interface{}, path string) (interface{}, error) {
				folder := d.(map[string]interface{})
				delete(folder, path)
				return folder, nil
			},

			Logger: log.TestWrapper(t),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Stop()

	path1 := filepath.Join(dir, "foo")
	if _, err = os.Create(path1); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path1, []byte("{\"number\":17}"), 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(1 * time.Second) // give the watcher time to parse

	/////////////////////////
	f1 := data.Get()
	if f1 == nil { //make sure it got data
		t.Error("data is nil")
	}
	fmt.Println("????????")
	fmt.Printf("%+v\n", f1)

	////////////////////////
	path2 := filepath.Join(dir, "bar")
	if err = os.Rename(path1, path2); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1 * time.Second)

	f2 := data.Get().(map[string]interface{})
	fmt.Println("????????")
	fmt.Printf("%+v\n", f2)
	if _, ok := f2[path1]; ok {
		t.Error("old data is still present")
	}
	if _, ok := f2[path2]; !ok {
		t.Error("new data is not present")
	}

}

func TestDirWatcherPathError(t *testing.T) {
	interval := time.Millisecond
	filewatcher.InitialReadInterval = interval
	round := interval * 20
	timeout := round * 4

	dir := t.TempDir()
	path := filepath.Join(dir, "foo")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := dirwatcher.New(
		ctx,
		dirwatcher.Config{
			Path: path,
			Parser: func(f io.Reader) (interface{}, error) {
				reader := f.(limitopen.ReadCloser)
				var data map[string]interface{}
				folder := make(map[string]interface{})
				err := json.NewDecoder(f).Decode(&data)

				folder[reader.Path] = data
				return folder, err
			},

			Adder: func(d interface{}, file interface{}) (interface{}, error) {
				folder := d.(map[string]interface{})

				for key, value := range file.(map[string]interface{}) {
					folder[key] = value
				}
				return folder, nil

			},

			Remover: func(d interface{}, path string) (interface{}, error) {
				folder := d.(map[string]interface{})
				delete(folder, path)
				return folder, nil
			},
			Logger: log.TestWrapper(t),
		},
	)
	if err == nil {
		t.Error("Expected dirwatcher path error, got nil.")
	}
}
