package secrets_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
)

func BenchmarkStoreMiddlewares(b *testing.B) {
	dir, err := ioutil.TempDir("", "secret_test_")
	if err != nil {
		b.Fatal(err)
	}

	tmpFile, err := ioutil.TempFile(dir, "secrets.json")
	if err != nil {
		b.Fatal(err)
	}
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
					secrets.NewStore(context.Background(), tmpFile.Name(), log.TestWrapper(b), middlewares...)
				}
			},
		)
	}
}
