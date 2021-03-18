package healthcheck

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/httpbp"
	"github.com/reddit/baseplate.go/internal/gen-go/reddit/baseplate"
	"github.com/reddit/baseplate.go/thriftbp"
)

const (
	defaultTimeout = time.Second
	maxHTTPBody    = 4096
)

// Run runs healthcheck.
//
// It returns 0 to indicate success,
// and non-zero to indicate failure.
//
// Your main function usually should look like:
//
//     func main() {
//       os.Exit(healthcheck.Run())
//     }
func Run() (ret int) {
	if err := RunArgs(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return -1
	}
	fmt.Println("OK!")
	return 0
}

// Actual value type: checker
var checkers = map[string]interface{}{
	"thrift": checker(checkThrift),
	"wsgi":   checker(checkHTTP),
	"http":   checker(checkHTTP),
}

// Actual value type: baseplate.IsHealthyProbe
var probes = map[string]interface{}{
	"readiness": baseplate.IsHealthyProbe_READINESS,
	"liveness":  baseplate.IsHealthyProbe_LIVENESS,
	"startup":   baseplate.IsHealthyProbe_STARTUP,
}

// RunArgs is the more customizable/testable version of Run.
//
// In production code it expects you to pass in os.Args as the arg.
func RunArgs(args []string) error {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	addr := fs.String(
		"endpoint",
		"localhost:9090",
		`The endpoint to find the service on, in "host:port" format without schema.`,
	)
	timeout := fs.Duration(
		"timeout",
		defaultTimeout,
		"The timeout for this healthcheck.",
	)
	check := oneof{
		choices: checkers,
		value:   "thrift",
	}
	fs.Var(
		&check,
		"type",
		fmt.Sprintf("The protocol of the service to check, one of %s.", check.choicesString()),
	)
	probe := oneof{
		choices: probes,
		value:   "readiness",
	}
	fs.Var(
		&probe,
		"probe",
		fmt.Sprintf("The probe to check, one of %s.", probe.choicesString()),
	)
	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("failed to parse args: %w", err)
	}
	return check.getValue().(checker)(
		*addr,
		probe.getValue().(baseplate.IsHealthyProbe),
		*timeout,
	)
}

type checker func(addr string, probe baseplate.IsHealthyProbe, timeout time.Duration) error

func checkThrift(addr string, probe baseplate.IsHealthyProbe, timeout time.Duration) error {
	cfg := thriftbp.ClientPoolConfig{
		Addr:               addr,
		InitialConnections: 1,
		MaxConnections:     5,
		ConnectTimeout:     timeout,
		SocketTimeout:      timeout,
	}
	pool, err := thriftbp.NewCustomClientPool(
		cfg,
		thriftbp.SingleAddressGenerator(addr),
		thrift.NewTHeaderProtocolFactoryConf(cfg.ToTConfiguration()),
	)
	if err != nil {
		return fmt.Errorf("failed to create thrift client pool: %w", err)
	}
	defer pool.Close()
	client := baseplate.NewBaseplateServiceV2Client(pool)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ret, err := client.IsHealthy(ctx, &baseplate.IsHealthyRequest{
		Probe: &probe,
	})
	if err != nil {
		return fmt.Errorf("thrift IsHealthy request failed: %w", err)
	}
	if !ret {
		return errors.New("thrift IsHealthy returned false")
	}
	return nil
}

func checkHTTP(addr string, probe baseplate.IsHealthyProbe, timeout time.Duration) error {
	client := http.Client{
		Timeout: timeout,
	}
	url := fmt.Sprintf(`http://%s/health?type=%v`, addr, probe)
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer httpbp.DrainAndClose(resp.Body)
	clientErr := httpbp.ClientErrorFromResponse(resp)
	if clientErr != nil {
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPBody))
		if err != nil {
			return fmt.Errorf(
				"http client error: %w, failed to read body: %v",
				clientErr,
				err,
			)
		}
		return fmt.Errorf(
			"http client error: %w, body: %s",
			clientErr,
			body,
		)
	}
	return nil
}
