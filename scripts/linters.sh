#!/bin/sh

DIR=$(dirname "$0")/..
cd "$DIR" || exit 1

FILES=$(find . -name "*.go" | grep -v -e "\/gen-go\/")
FAILED=0

for FILE in $FILES; do
  FMT=$(gofmt -s -d "$FILE")
  if [ -n "$FMT" ]; then
    echo "gofmt:"
    echo "$FILE:"
    echo "$FMT"
    FAILED=1
  fi
done

VET=$(go vet ./...)
if [ -n "$VET" ]; then
  echo "go vet:"
  echo "$VET"
  FAILED=1
fi

STATICCHECK=$(staticcheck ./...)
if [ -n "$STATICCHECK" ]; then
  echo "$(staticcheck --version):"
  echo "$STATICCHECK"
  FAILED=1
fi

exit $FAILED
