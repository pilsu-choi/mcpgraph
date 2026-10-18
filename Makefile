# The race detector needs cgo (a C compiler) at test time. The shipped
# binaries are still built with CGO_ENABLED=0. If no C compiler is found
# locally we run the tests without -race; CI always has one.
RACE := $(shell command -v gcc >/dev/null 2>&1 && echo -race)

.PHONY: build test lint scan release demo

build:
	CGO_ENABLED=0 go build ./cmd/...

test:
ifeq ($(RACE),)
	@echo "warning: gcc not found, running tests without -race"
	go test ./...
else
	CGO_ENABLED=1 go test -race ./...
endif

lint:
	golangci-lint run

scan:
	go run ./cmd/mcpgraph scan

release:
	goreleaser build --snapshot --clean

# Regenerates docs/logs and docs/images from real runs.
demo:
	scripts/demo.sh
