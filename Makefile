VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS = -X github.com/jeefy/convergence/internal/version.Version=$(VERSION) \
          -X github.com/jeefy/convergence/internal/version.Commit=$(COMMIT) \
          -X github.com/jeefy/convergence/internal/version.Date=$(DATE)

.PHONY: all build test lint clean

all: build

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/convergence ./cmd/convergence

test:
	go test -v ./...

lint:
	go vet ./...

clean:
	rm -rf bin dist
