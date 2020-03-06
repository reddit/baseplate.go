#!/bin/sh

bazel --bazelrc=.bazelrc.ci test //...:all

if [ $? -ne 0 ]; then
  # Test failed, print out all test logs then exit with non-zero code.
  find bazel-testlogs/ -name "test.xml" -print -exec grep -C 1 Failed {} \;
  exit 1
fi
