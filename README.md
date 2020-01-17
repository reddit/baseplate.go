# baseplate.go
[Baseplate](https://github.com/reddit/baseplate.py) implemented in go

## IDE/Editor Setup

See [here](Editor.md).

## Thrift generated files

The `internal/gen-go/` directory contains thrift generated files,
with `*-remote` directories removed.
They are excluded from the linter.
DO NOT EDIT.

They were generated with thrift compiler 0.13.0 and
[`baseplate.thrift`][baseplate.thrift] using command under `internal/`:

```
thrift --gen go:package_prefix=github.com/reddit/baseplate.go/ path/to/baseplate.thrift
```

They are needed by [`edgecontext`][edgecontext] package.
We did not include `baseplate.thrift` file into this repo to avoid duplications.
This directory will be regenerated when either thrift compiler or
`baseplate.thrift` changed significantly.

[baseplate.thrift]: https://github.com/reddit/baseplate.py/blob/d6c6a03841862d7803bffbfbcaf5d6bf9357589e/baseplate/thrift/baseplate.thrift

[edgecontext]: https://godoc.org/github.com/reddit/baseplate.go/edgecontext
