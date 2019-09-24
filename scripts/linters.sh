#!/bin/sh

DIR=$(dirname $0)/..
cd $DIR

FILES=`find . -name "*.go" | grep -v -e "^\.\/vendor\/"`
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

VET=`go vet -mod=vendor ./... 2>&1`
if [ -n "$VET" ]; then
  echo "go vet\n$VET"
  FAILED=1
fi

exit $FAILED
