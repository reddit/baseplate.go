.PHONY: test

GO=go
GO_TEST=$(GO) test -race ./...

test:
	$(GO_TEST)
