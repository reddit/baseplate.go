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

promauto_new=$(git grep "\<promauto.New" | grep -v "linters.sh" | grep -v "//.*promauto.New")
if [ -n "$promauto_new" ]; then
  echo "*** Uses of promauto should use With(prometheusbpint.GlobalRegistry):"
  echo "$promauto_new"
  FAILED=1
fi

prometheus_register=$(git grep -w "prometheus.Register" | grep -v "linters.sh" | grep -v "//.*prometheus.Register")
if [ -n "$prometheus_register" ]; then
  echo "*** Uses of prometheus.Register should use prometheusbpint.GlobalRegistry.Register instead:"
  echo "$prometheus_register"
  FAILED=1
fi

exit $FAILED
