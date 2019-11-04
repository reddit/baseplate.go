#!/bin/sh

DIR=$(dirname $0)/..
cd $DIR

FILES=`find . -name "*.go"`
FAILED=0

for FILE in $FILES; do
  LINT=`golint $FILE`
  if [ -n "$LINT" ]; then
    echo "$FILE:\ngolint\n$LINT"
    FAILED=1
  fi

  FMT=`gofmt -d $FILE`
  if [ -n "$FMT" ]; then
    echo "$FILE:\ngofmt\n$FMT"
    FAILED=1
  fi
done

exit $FAILED
