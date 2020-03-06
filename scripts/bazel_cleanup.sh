#!/bin/sh

bazel run //:gazelle -- fix
bazel run //:gazelle -- update-repos -from_file=go.mod -prune -to_macro=external.bzl%go_dependencies
