.PHONY: test bazeltest gotest

BAZEL=bazel
BAZEL_TEST=$(BAZEL) test //...:all
BAZEL_CLEAN=$(BAZEL) clean
GO=go
GO_TEST=$(GO) test -race ./...

test:
	if [ -n "$(shell which $(BAZEL))" ]; \
		then $(BAZEL_TEST); \
		else $(GO_TEST); \
		fi

bazeltest:
	$(BAZEL_TEST)

gotest:
	if [ -n "$(shell which $(BAZEL))" ]; then $(BAZEL_CLEAN); fi
	$(GO_TEST)
