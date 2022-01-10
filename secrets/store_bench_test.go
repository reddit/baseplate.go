package secrets_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
)

func BenchmarkStoreMiddlewares(b *testing.B) {
	dir, err := os.MkdirTemp("", "secret_test_")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := os.CreateTemp(dir, "secrets.json")
	if err != nil {
		b.Fatal(err)
	}
	defer tmpFile.Close()
	tmpFile.Write([]byte(specificationExample))

	var middleware = func(next secrets.SecretHandlerFunc) secrets.SecretHandlerFunc {
		return func(sec *secrets.Secrets) {
			next(sec)
		}
	}

	for i := 0; i < 10; i++ {
		numOfMiddlewares := 1 << i

		middlewares := make([]secrets.SecretMiddleware, 0, numOfMiddlewares)

		for j := 0; j < numOfMiddlewares; j++ {
			middlewares = append(middlewares, middleware)
		}

		b.Run(
			fmt.Sprintf("number of middlewares %d", numOfMiddlewares),
			func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					secrets.NewStore(context.Background(), tmpFile.Name(), "vault", log.TestWrapper(b), middlewares...)
				}
			},
		)
	}
}
