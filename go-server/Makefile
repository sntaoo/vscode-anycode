# Makefile for AnyCode Go Server

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary name
BINARY_NAME=anycode-server
BINARY_UNIX=$(BINARY_NAME)_unix
BINARY_WINDOWS=$(BINARY_NAME).exe

# Build targets
.PHONY: all build clean test deps run help

all: clean deps build

build: 
	$(GOBUILD) -o $(BINARY_NAME) -v

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BINARY_WINDOWS) -v

build-all: build-linux build-windows

clean: 
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f $(BINARY_WINDOWS)

test: 
	$(GOTEST) -v ./...

test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out

deps:
	$(GOMOD) download
	$(GOMOD) verify

deps-update:
	$(GOMOD) tidy
	$(GOGET) -u ./...

run-stdio:
	$(GOBUILD) -o $(BINARY_NAME) && ./$(BINARY_NAME) -mode=stdio

run-tcp:
	$(GOBUILD) -o $(BINARY_NAME) && ./$(BINARY_NAME) -mode=tcp -addr=:4389

run-stdio-with-packages:
	$(GOBUILD) -o $(BINARY_NAME) && ./$(BINARY_NAME) -mode=stdio -packages=../

run-tcp-with-packages:
	$(GOBUILD) -o $(BINARY_NAME) && ./$(BINARY_NAME) -mode=tcp -addr=:4389 -packages=../

install:
	$(GOCMD) install

# Development helpers
dev-tools:
	$(GOGET) golang.org/x/tools/cmd/goimports
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint

lint:
	golangci-lint run

format:
	gofmt -s -w .
	goimports -w .

# Docker targets
docker-build:
	docker build -t anycode-server .

docker-run:
	docker run -p 4389:4389 anycode-server

# Release targets
release: clean deps test build-all
	mkdir -p release
	cp $(BINARY_UNIX) release/
	cp $(BINARY_WINDOWS) release/
	cp README.md release/
	tar -czf release/anycode-server-linux.tar.gz -C release $(BINARY_UNIX) README.md
	zip -j release/anycode-server-windows.zip release/$(BINARY_WINDOWS) release/README.md

test-languages:
	chmod +x scripts/test-languages.sh
	./scripts/test-languages.sh

help:
	@echo "Available targets:"
	@echo "  build                    - Build the binary"
	@echo "  build-linux              - Build for Linux"
	@echo "  build-windows            - Build for Windows"
	@echo "  build-all                - Build for all platforms"
	@echo "  clean                    - Clean build artifacts"
	@echo "  test                     - Run tests"
	@echo "  test-coverage            - Run tests with coverage"
	@echo "  test-languages           - Test language package functionality"
	@echo "  deps                     - Download dependencies"
	@echo "  deps-update              - Update dependencies"
	@echo "  run-stdio                - Build and run in stdio mode"
	@echo "  run-tcp                  - Build and run in TCP mode"
	@echo "  run-stdio-with-packages  - Build and run with external packages"
	@echo "  run-tcp-with-packages    - Build and run TCP mode with external packages"
	@echo "  install                  - Install the binary"
	@echo "  dev-tools                - Install development tools"
	@echo "  lint                     - Run linter"
	@echo "  format                   - Format code"
	@echo "  docker-build             - Build Docker image"
	@echo "  docker-run               - Run Docker container"
	@echo "  release                  - Create release packages"
	@echo "  help                     - Show this help"