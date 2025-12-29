BUILD_DIR=.
BINARY_NAME=tzsp_server
VERSION=$(shell cat .version 2>/dev/null || echo "dev")
LDFLAGS=-s -w -X github.com/pavelkim/tzsp_server/internal/version.Version=$(VERSION)

.PHONY: all build clean run test deps

all: deps build

deps:
	go mod download
	go mod tidy

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/tzsp_server

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/tzsp_server

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/tzsp_server
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/tzsp_server

build-all: build-linux build-darwin

run: build
	./$(BINARY_NAME)

test:
	go test -v -race -coverprofile=coverage.out ./...

coverage: test
	go tool cover -html=coverage.out

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-*
	rm -f *.pcap *.log coverage.out
	go clean

install: build
	cp $(BINARY_NAME) /usr/local/bin/

fmt:
	go fmt ./...

lint:
	golangci-lint run

help:
	@echo "TZSP Server Build System"
	@echo ""
	@echo "Targets:"
	@echo "  all          - Download dependencies and build"
	@echo "  deps         - Download Go dependencies"
	@echo "  build        - Build for current platform"
	@echo "  build-linux  - Build for Linux AMD64"
	@echo "  build-darwin - Build for macOS (AMD64 and ARM64)"
	@echo "  build-all    - Build for all platforms"
	@echo "  run          - Build and run the server"
	@echo "  test         - Run tests with race detector"
	@echo "  coverage     - Generate test coverage report"
	@echo "  clean        - Remove build artifacts"
	@echo "  install      - Install binary to /usr/local/bin"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  help         - Show this help message"
