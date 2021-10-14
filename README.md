# baseplate.go

[Baseplate][baseplate.py] implemented in go.

## Documentation

[Code documentation][godev]

## IDE/Editor setup

See [here](Editor.md).

## Code style guide

See [here](Style.md).

## Thrift generated files

The `internal/gen-go/` directory contains thrift generated files,
with `*-remote` directories removed.
They are excluded from the linter.
DO NOT EDIT.

They were generated with [thrift compiler v0.14.1][thrift-version] against
[`baseplate.thrift`][baseplate.thrift]
using the following commands under `internal/`:

```
thrift --gen go:package_prefix=github.com/reddit/baseplate.go/ path/to/baseplate.thrift
find gen-go -depth -name "*-remote" -type d -exec rm -Rf {} \;
```

They are needed by some of the Baseplate.go packages.
We did not include those thrift files into this repo to avoid duplications.
This directory will be regenerated when either thrift compiler or the thrift
files changed significantly.

[baseplate.py]: https://github.com/reddit/baseplate.py

[baseplate.thrift]: https://github.com/reddit/baseplate.py/blob/b1e1dbddd0994c2b2a36c8c456fe8f08dadf1c9d/baseplate/thrift/baseplate.thrift

[godev]: https://pkg.go.dev/github.com/reddit/baseplate.go

[thrift-version]: https://github.com/apache/thrift/releases/tag/v0.14.1
