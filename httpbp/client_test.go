package httpbp

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var Epsilon = 500 * time.Millisecond

func TestMaxConcurrency(t *testing.T) {
	port, closer, err := Serve()
	if err != nil {
		panic(err)
	}
	defer closer.Close()
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	client := NewClient(NewTypicalClientConfig())
	var errors uint64 = 0
	n := 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := client.Get(url)
			if err != nil {
				atomic.AddUint64(&errors, 1)
				fmt.Println(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if 50 != int(errors) {
		t.Errorf("Expected 50 errors, got %d", errors)
	}
}

func TestCircuitBreaking(t *testing.T) {
	port, closer, err := Serve()
	if err != nil {
		panic(err)
	}
	defer closer.Close()
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	client := NewClient(NewTypicalClientConfig())
	n := 100
	for i := 0; i < n; i++ {
		_, err = client.Get(url + "error")
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("Expected a 500 error")
		}
	}
	for i := 0; i < n; i++ {
		_, err = client.Get(url + "error")
		if !strings.Contains(err.Error(), "circuit breaker is open") {
			t.Errorf("Expected circuit breaker to trip")
		}
	}
}

func Serve() (int, io.Closer, error) {
	// Serves a simple http server for testing on a random port.  Returns the port so you
	// know where to reach it!
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, nil, err
	}
	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		go func() {
			time.Sleep(Epsilon)
			w.Write([]byte("hello world\n"))
		}()
	})
	handler.HandleFunc("/error", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(500)
	})
	go http.Serve(listener, handler)
	time.Sleep(Epsilon)
	return listener.Addr().(*net.TCPAddr).Port, listener, nil
}
