#!/bin/sh

bazel run //:gazelle -- update-repos -from_file=go.mod -prune -to_macro=external.bzl%go_dependencies
bazel run //:gazelle -- fix

# Temporary solution to workaround https://github.com/bazelbuild/bazel-gazelle/issues/990
git checkout -- go.sum
