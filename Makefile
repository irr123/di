.PHONY: all
all: fmt lint test

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint:
	go vet ./...

.PHONY: test
test:
	go test -v ./...
