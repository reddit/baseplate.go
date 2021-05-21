package httpbp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"

	baseplate "github.com/reddit/baseplate.go"
	"github.com/reddit/baseplate.go/errorsbp"
	"github.com/reddit/baseplate.go/log"
)

var allHTTPMethods = map[string]bool{
	http.MethodConnect: true,
	http.MethodDelete:  true,
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
	http.MethodPatch:   true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodTrace:   true,
}

// EndpointRegistry is the minimal interface needed by a Baseplate HTTP server for
// the underlying HTTP server.
//
// *http.ServeMux implements this interface and is the default EndpointRegistry
// used by NewBaseplateServer.
type EndpointRegistry interface {
	http.Handler

	Handle(pattern string, handler http.Handler)
}

var (
	_ EndpointRegistry = (*http.ServeMux)(nil)
)

type httpHandlerFactory struct {
	middlewares []Middleware
}

func (f httpHandlerFactory) NewHandler(endpoint Endpoint) http.Handler {
	wrappers := make([]Middleware, 0, len(f.middlewares)+len(endpoint.Middlewares)+1)
	wrappers = append(wrappers, SupportedMethods(endpoint.Methods[0], endpoint.Methods[1:]...))
	wrappers = append(wrappers, f.middlewares...)
	wrappers = append(wrappers, endpoint.Middlewares...)
	return NewHandler(endpoint.Name, endpoint.Handle, wrappers...)
}

// Pattern is the pattern passed to a EndpointRegistry when registering an
// Endpoint.
type Pattern string

// Endpoint holds the values needed to create a new HandlerFunc.
type Endpoint struct {
	// Name is required, it is the "name" of the endpoint that will be passed
	// to any Middleware wrapping the HandlerFunc.
	Name string

	// Methods is the list of HTTP methods that the endpoint supports.  Methods
	// must have at least one entry and all entries must be valid HTTP methods.
	//
	// Method names should be in all upper case.
	// Use the http.Method* constants from "net/http" for the values in this slice
	// to ensure that you are using methods that are supported and in the format
	// we expect.
	// If you add http.MethodGet, http.MethodHead will be supported automatically.
	Methods []string

	// Handle is required, it is the base HandlerFunc that will be wrapped
	// by any Middleware.
	Handle HandlerFunc

	// Middlewares is an optional list of additional Middleware to wrap the
	// given HandlerFunc.
	Middlewares []Middleware
}

// Validate checks for input errors on the Endpoint and returns an error
// if any exist.
func (e Endpoint) Validate() error {
	var err errorsbp.Batch
	if e.Name == "" {
		err.Add(errors.New("httpbp: Endpoint.Name must be non-empty"))
	}
	if e.Handle == nil {
		err.Add(errors.New("httpbp: Endpoint.Handle must be non-nil"))
	}
	if len(e.Methods) == 0 {
		err.Add(errors.New("httpbp: Endpoint.Methods must be non-empty"))
	} else {
		for _, method := range e.Methods {
			if !allHTTPMethods[method] {
				err.Add(fmt.Errorf("httpbp: Endpoint.Methods contains an invalid value: %q", method))
			}
		}
	}
	return err.Compile()
}

// ServerArgs defines all of the arguments used to create a new HTTP
// Baseplate server.
type ServerArgs struct {
	// Baseplate is a required argument to NewBaseplateServer and must
	// be non-nil.
	Baseplate baseplate.Baseplate

	// Endpoints is the mapping of endpoint patterns to Endpoint objects that
	// the Server will handle.
	//
	// While endpoints is not technically required, if none are provided, your
	// server will not handle any Endpoints.
	Endpoints map[Pattern]Endpoint

	// EndpointRegistry is an optional argument that can be used to customize
	// the EndpointRegistry used by the Baseplate HTTP server.
	//
	// Defaults to a new *http.ServeMux.
	//
	// Most servers will not need to set this, it has been provided for cases
	// where you need to use something other than http.ServeMux.
	//
	// If you do customize this, you should use a new EndpointRegistry and
	// register your endpoints using server.Handle rather than pre-registering
	// endpoints.  Any endpoints registered in other ways will not be
	// httpbp.HandlerFunc-s and will not be wrapped in any Middleware.
	EndpointRegistry EndpointRegistry

	// Middlewares is optional, additional Middleware that will wrap any
	// HandlerFuncs registered to the server using server.Handle.
	Middlewares []Middleware

	// OnShutdown is an optional list of functions that can be run when
	// server.Stop is called.
	OnShutdown []func()

	// TrustHandler is an optional HeaderTrustHandler that will be used
	// by the default Middleware to determine if we can trust the HTTP
	// headers that can be used to initialize spans/edge request contexts.
	//
	// Defaults to NeverTrustHeaders.
	TrustHandler HeaderTrustHandler

	// Logger is an optional arg to be called when the InjectEdgeRequestContext
	// middleware failed to parse the edge request header for any reason.
	Logger log.Wrapper
}

// ValidateAndSetDefaults checks the ServerArgs for any errors and sets any
// default values.
//
// ValidateAndSetDefaults does not generally need to be called manually but can
// be used for testing purposes.  It is called as a part of setting up a new
// Baseplate server.
func (args ServerArgs) ValidateAndSetDefaults() (ServerArgs, error) {
	var inputErrors errorsbp.Batch
	if args.Baseplate == nil {
		inputErrors.Add(errors.New("argument Baseplate must be non-nil"))
	}
	for _, endpoint := range args.Endpoints {
		inputErrors.Add(endpoint.Validate())
	}
	if args.EndpointRegistry == nil {
		args.EndpointRegistry = http.NewServeMux()
	}
	if args.TrustHandler == nil {
		args.TrustHandler = NeverTrustHeaders{}
	}
	return args, inputErrors.Compile()
}

// SetupEndpoints calls ValidateAndSetDefaults and registeres the Endpoints
// in args to the EndpointRegistry in args and returns the fully setup ServerArgs.
//
// SetupEndpoints does not generally need to be called manually but can
// be used for testing purposes.  It is called as a part of setting up a new
// Baseplate server.
func (args ServerArgs) SetupEndpoints() (ServerArgs, error) {
	args, err := args.ValidateAndSetDefaults()
	if err != nil {
		return args, err
	}

	wrappers := DefaultMiddleware(DefaultMiddlewareArgs{
		TrustHandler:    args.TrustHandler,
		EdgeContextImpl: args.Baseplate.EdgeContextImpl(),
		Logger:          args.Logger,
	})
	wrappers = append(wrappers, args.Middlewares...)

	factory := httpHandlerFactory{middlewares: wrappers}
	for pattern, endpoint := range args.Endpoints {
		args.EndpointRegistry.Handle(string(pattern), factory.NewHandler(endpoint))
	}
	return args, nil
}

// NewBaseplateServer returns a new HTTP implementation of a Baseplate
// server with the given ServerArgs.
//
// The Endpoints given in the ServerArgs will be wrapped using the
// default Baseplate Middleware as well as any additional Middleware
// passed in.
func NewBaseplateServer(args ServerArgs) (baseplate.Server, error) {
	args, err := args.SetupEndpoints()
	if err != nil {
		return nil, err
	}

	srv := &http.Server{
		Addr:         args.Baseplate.GetConfig().Addr,
		Handler:      args.EndpointRegistry,
		ReadTimeout:  args.Baseplate.GetConfig().Timeout,
		WriteTimeout: args.Baseplate.GetConfig().Timeout,
	}
	for _, f := range args.OnShutdown {
		srv.RegisterOnShutdown(f)
	}
	return &server{args.Baseplate, srv}, nil
}

type server struct {
	bp  baseplate.Baseplate
	srv *http.Server
}

func (s server) Baseplate() baseplate.Baseplate {
	return s.bp
}

func (s server) Serve() error {
	// ListenAndServe always returns a non-nil error, http.ErrServerClosed is the
	// "expected" error for it to return after being shutdown.
	//
	// https://golang.org/pkg/net/http/#Server.ListenAndServe
	err := s.srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func (s server) Close() error {
	return s.srv.Shutdown(context.TODO())
}

// NewTestBaseplateServer returns a new HTTP implementation of a Baseplate
// server with the given ServerArgs that uses a Server from httptest rather than
// a real server.
//
// The underlying httptest.Server is started when the the test BaseplateServer
// is created and does not need to be started manually.
// It is closed by calling Close, Close should not be called more than once.
// Serve does not need to be called but will wait until Close is called to exit
// if it is called.
func NewTestBaseplateServer(args ServerArgs) (baseplate.Server, *httptest.Server, error) {
	args, err := args.SetupEndpoints()
	if err != nil {
		return nil, nil, err
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	ts := httptest.NewServer(args.EndpointRegistry)
	return &testServer{
		bp:         args.Baseplate,
		onShutdown: args.OnShutdown,
		srv:        ts,
		wg:         wg,
	}, ts, nil
}

type testServer struct {
	bp         baseplate.Baseplate
	onShutdown []func()
	srv        *httptest.Server
	wg         *sync.WaitGroup
}

func (s *testServer) Baseplate() baseplate.Baseplate {
	return s.bp
}

func (s *testServer) Serve() error {
	s.wg.Wait()
	return nil
}

func (s *testServer) Close() error {
	s.srv.Close()
	for _, cb := range s.onShutdown {
		cb()
	}
	s.wg.Done()
	return nil
}

var (
	_ baseplate.Server = (*testServer)(nil)
)
