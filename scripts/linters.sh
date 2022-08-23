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

# We run all go vet checks for all packages except thriftbp.
# For thriftbp we have to disable stdmethods as we have an implementatino of
# thrift.TProtocol which has different ReadByte and WriteByte signature that
# go vet with stdmethods will complain.
#
# TODO: Once thrift 0.17.0 is released we can remove this implementation and
# revert back to simply run all go vet checks for all packages with:
#
#     VET=$(go vet ./...)
thriftbp_pattern="thriftbp$"
packages_sans_thriftbp=$(go list ./... | grep -v "$thriftbp_pattern")
packages_only_thriftbp=$(go list ./... | grep "$thriftbp_pattern")
VET=$(go vet $packages_sans_thriftbp; go vet -stdmethods=false $packages_only_thriftbp)
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

if grep -ri 'promauto.New' $(find . -name "*.go" -not -name "*_test.go") | grep -v "//.*promauto.New"; then
  echo "*** Uses of promauto above should use With(internalv2compat.GlobalRegistry)"
fi
if grep -ri 'prometheus.Register' $(find . -name "*.go" -not -name "*_test.go") | grep -v "//.*prometheus.Register"; then
  echo "*** Uses of prometheus.Register above should use internalv2compat.GlobalRegistry.Register instead"
fi


exit $FAILED
