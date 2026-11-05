VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS = -X github.com/jeefy/convergence/internal/version.Version=$(VERSION) \
          -X github.com/jeefy/convergence/internal/version.Commit=$(COMMIT) \
          -X github.com/jeefy/convergence/internal/version.Date=$(DATE)

.PHONY: all build test test-e2e test-e2e-copilot e2e-test test-e2e-cluster lint fmt clean

all: build

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/convergence ./cmd/convergence

test:
	go test -v ./...

test-e2e: build
	./tests/test_e2e_local.sh

test-e2e-copilot: build
	./tests/test_e2e_copilot.sh

e2e-test: test-e2e-copilot

test-e2e-cluster: build
	./tests/test_e2e_cluster.sh

lint:
	golangci-lint run ./...
	go vet ./...
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

fmt:
	gofmt -w .
	golangci-lint fmt ./...

clean:
	rm -rf bin dist
