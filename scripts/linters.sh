#!/bin/sh

DIR=$(dirname $0)/..
cd $DIR

FILES=`find . -name "*.go" | grep -v -e "\/gen-go\/"`
FAILED=0

for FILE in $FILES; do
  LINT=`golint $FILE`
  if [ -n "$LINT" ]; then
    echo "$FILE:\ngolint\n$LINT"
    FAILED=1
  fi

  FMT=`gofmt -s -d $FILE`
  if [ -n "$FMT" ]; then
    echo "$FILE:\ngofmt\n$FMT"
    FAILED=1
  fi
done

STATICCHECK=`staticcheck ./...`
if [ -n "$STATICCHECK" ]; then
  echo "staticcheck:\n$STATICCHECK"
  FAILED=1
fi

exit $FAILED
