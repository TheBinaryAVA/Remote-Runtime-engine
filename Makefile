.PHONY: all build build-all test test-linux docker-build compose-up compose-down clean

BINARY_ENGINE=speedcode-engine
BINARY_API=speedcode-api
BINARY_WORKER=speedcode-worker
RUNNER_IMAGE=speedcode-runner:latest

all: build-all test

build:
	@echo "==> Building engine binary..."
	go build -ldflags="-s -w" -o bin/$(BINARY_ENGINE) cmd/engine/main.go

build-all:
	@echo "==> Building all binaries (engine, api, worker)..."
	go build -ldflags="-s -w" -o bin/$(BINARY_ENGINE) cmd/engine/main.go
	go build -ldflags="-s -w" -o bin/$(BINARY_API) cmd/api/main.go
	go build -ldflags="-s -w" -o bin/$(BINARY_WORKER) cmd/worker/main.go

test:
	@echo "==> Running complete test suite..."
	go test -v -race -cover ./...

test-linux:
	@echo "==> Verifying Linux cross-compilation..."
	GOOS=linux GOARCH=amd64 go build -o /dev/null ./...

docker-build:
	@echo "==> Building container images..."
	docker build -t $(RUNNER_IMAGE) .

compose-up:
	@echo "==> Launching distributed cluster (Redis, API Gateway, Workers)..."
	docker-compose up -d --build

compose-down:
	@echo "==> Stopping distributed cluster..."
	docker-compose down

clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf bin/ /tmp/speedcode
