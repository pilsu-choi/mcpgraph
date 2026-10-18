# The race detector needs cgo (a C compiler) at test time. The shipped
# binaries are still built with CGO_ENABLED=0. If no C compiler is found
# locally we run the tests without -race; CI always has one.
RACE := $(shell command -v gcc >/dev/null 2>&1 && echo -race)

.PHONY: build test bench lint scan release demo docs

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
	@echo "make demo: not implemented yet (milestone 10); see make docs" >&2
	@exit 1

# Regenerates docs/logs and docs/images from real runs.
docs:
	scripts/docs.sh
