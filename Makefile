### test: Run all tests
.PHONY: test
test:
	go test ./...

### test-v: Run all tests (verbose)
.PHONY: test-v
test-v:
	go test -v ./...

### build: Build all packages
.PHONY: build
build:
	go build ./...

### clean: Clean build artifacts
.PHONY: clean
clean:
	go clean ./...

### fmt: Format all Go files
.PHONY: fmt
fmt:
	gofmt -w -s .

### help: List all available targets
.PHONY: help
help:
	@grep '^### ' Makefile | sed 's/^### //'
