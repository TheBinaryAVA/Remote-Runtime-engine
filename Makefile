.PHONY: all build test test-linux docker-build clean

BINARY_NAME=speedcode-engine
RUNNER_IMAGE=speedcode-runner:latest

all: build test

build:
	@echo "==> Building engine binary..."
	go build -ldflags="-s -w" -o bin/$(BINARY_NAME) cmd/engine/main.go

test:
	@echo "==> Running test suite..."
	go test -v -race -cover ./...

test-linux:
	@echo "==> Verifying Linux cross-compilation..."
	GOOS=linux GOARCH=amd64 go build -o /dev/null ./...

docker-build:
	@echo "==> Building isolated sandbox container image..."
	docker build -t $(RUNNER_IMAGE) .

clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf bin/ /tmp/speedcode
