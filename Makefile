# The race detector needs cgo (a C compiler) at test time. The shipped
# binaries are still built with CGO_ENABLED=0. If no C compiler is found
# locally we run the tests without -race; CI always has one.
RACE := $(shell command -v gcc >/dev/null 2>&1 && echo -race)

.PHONY: build test bench lint scan release demo demo-down docs

build:
	CGO_ENABLED=0 go build ./cmd/...

test:
ifeq ($(RACE),)
	@echo "warning: gcc not found, running tests without -race"
	go test ./...
else
	CGO_ENABLED=1 go test -race ./...
endif

bench:
	go test -run=^$$ -bench=. -benchmem ./internal/policy/...

lint:
	golangci-lint run

scan:
	go run ./cmd/mcpgraph scan

release:
	goreleaser build --snapshot --clean

# The DROP TABLE demo (milestone 10).
demo:
	demo/run.sh

# Removes the demo database container and volume if a demo run was interrupted.
demo-down:
	demo/down.sh

# Regenerates docs/logs and docs/images from real runs.
docs:
	scripts/docs.sh
