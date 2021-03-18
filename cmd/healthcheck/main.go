package main

import (
	"os"

	"github.com/reddit/baseplate.go/cmd/lib/healthcheck"
)

func main() {
	os.Exit(healthcheck.Run())
}
