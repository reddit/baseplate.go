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

They were generated with thrift compiler 0.13.0 and
[`baseplate.thrift`][baseplate.thrift] using command under `internal/`:

```
thrift --gen go:package_prefix=github.com/reddit/baseplate.go/ path/to/baseplate.thrift
```

They are needed by [`edgecontext`][edgecontext] package.
We did not include `baseplate.thrift` file into this repo to avoid duplications.
This directory will be regenerated when either thrift compiler or
`baseplate.thrift` changed significantly.

## Bazel support

This project also comes with *optional* [Bazel][bazel] support.
It's optional as in you can totally ignore Bazel and still use the go toolchain,
but the added support will make it easier for projects using Bazel to add this
project as a dependency.

When you made a change to `go.mod` file,
please run the following command to reflect the changes in Bazel:

```
bazel run //:gazelle -- update-repos -from_file=go.mod -prune
```

Or just use the script we used in CI:

```
./scripts/bazel_cleanup.sh
```

To run tests via Bazel, use the following command:

```
bazel test //...:all
```


[baseplate.py]: https://github.com/reddit/baseplate.py

[baseplate.thrift]: https://github.com/reddit/baseplate.py/blob/d6c6a03841862d7803bffbfbcaf5d6bf9357589e/baseplate/thrift/baseplate.thrift

[edgecontext]: https://godoc.org/github.com/reddit/baseplate.go/edgecontext

[bazel]: https://bazel.build/

[godev]: https://pkg.go.dev/github.com/reddit/baseplate.go

