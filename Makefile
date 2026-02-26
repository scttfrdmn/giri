.PHONY: build build-arena test test-arena test-all lint clean install

# Default build (works without GOEXPERIMENT=arenas)
build:
	go build ./...

# Build with arena support
build-arena:
	GOEXPERIMENT=arenas go build ./...

# Install the giri binary
install:
	go install ./cmd/giri

install-arena:
	GOEXPERIMENT=arenas go install ./cmd/giri

# Tests without arena experiment (core packages only)
test:
	go test ./pkg/shadow/... ./pkg/interpreter/... ./pkg/detector/... ./pkg/scheduler/... ./pkg/report/...

# Tests with arena experiment (includes arena-specific testdata)
test-arena:
	GOEXPERIMENT=arenas go test ./...

# Run both test suites
test-all: test test-arena

# Tests with race detector
test-race:
	go test -race ./pkg/...
	GOEXPERIMENT=arenas go test -race ./...

# Vet
vet:
	go vet ./...
	GOEXPERIMENT=arenas go vet ./...

# Tidy
tidy:
	go mod tidy

# Clean
clean:
	rm -f giri
	go clean ./...
