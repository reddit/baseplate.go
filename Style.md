# Baseplate.go Coding Style Guide

## Use tools

Make sure your code passes the following tools:

- `go vet`
- `gofmt -s`
- `golint` (via `go install golang.org/x/lint/golint@latest`)
- `staticcheck` (via `go install honnef.co/go/tools/cmd/staticcheck@latest`)

See our [IDE/Editor setup doc](Editor.md) for ways to doing that automatically.

### Exceptions

Generated code (e.g. code generated by thrift compiler) are exempted from this
rule.

## Import groups

Group your imports into the following 3 groups, in that order,
separated by blank lines:

1. Standard library packages
2. Third-party packages
   (non-stdlib packages without `github.com/reddit/baseplate.go` prefix)
3. Packages from the same module
   (packages with `github.com/reddit/baseplate.go` prefix)

Example:

```go
import (
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/reddit/baseplate.go/log"
)
```

### Exceptions

In whole file examples
(see the end of https://pkg.go.dev/testing?tab=doc#hdr-Examples),
merge Groups 2&3.
The reason is that whole file examples are presented in package users' point of
view,
and in that point of view everything under `github.com/reddit/baseplate.go` are
also third-party packages.

Please note that this exception does not include tests and normal examples.

## Rename imports

When an imported package name is different from the last element of the import
path, always rename import it to make it explicit.

Example:

```go
import (
	jwt "gopkg.in/dgrijalva/jwt-go.v3"
	opentracing "github.com/opentracing/opentracing-go"
)
```

### Exceptions

As recommended by go modules version design,
in the example of `"github.com/go-redis/redis/v7"`,
`v7` is the major version number,
so we consider `redis` as the actual last element of import path.
As a result there's no need to rename import it.

## One-per-line style

When putting all args to a function signature/call to a single line makes the
line too long, Use one-per-line style with closing parenthesis on the next line.
Please note that in function signatures one-per-line means one group per line
instead of one arg per line.

Example:

```go
func foo (
	arg1, arg2 int,
	arg3 string,
	arg4 func(),
) error {
	// Function body
	return nil
}

foo(
	1,
	2,
	"string",
	func() {
		// Function body
	},
)
```

When writing slice/map literals in one-per-line style,
also make sure to put the closing curly bracket on the next line.

Example:

```go
slice := []int{
	1,
	2,
	3,
}

myMap := map[int]string{
	1: "one",
	2: "two",
}
```

### Exceptions

When using slice literal/args as key-value map,
use two-per-line style to make sure that we always have them in pair.

Example:

```go
log.Errorw(
	"Something went wrong!",
	"err", err,
	"endpoint", "myEndpoint",
)
```

## Other resources

For things not covered above,
use your best judgement and follow industry best practises.
Some recommended resources are:

- [Effective Go](https://golang.org/doc/effective_go.html)
- [CodeReviewComments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
