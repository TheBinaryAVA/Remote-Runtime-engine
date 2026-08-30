# ========================================================
# Stage 1: Build Go Binaries
# ========================================================
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/speedcode-engine cmd/engine/main.go && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/speedcode-api cmd/api/main.go && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/speedcode-worker cmd/worker/main.go

# ========================================================
# Stage 2: Runtime Sandbox Base (C++, Python3, unprivileged user)
# ========================================================
FROM ubuntu:22.04 AS base-runtime

ENV DEBIAN_FRONTEND=noninteractive
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    g++ \
    python3 \
    python3-minimal \
    libstdc++6 \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -g 1001 speedcode && \
    useradd -u 1001 -g speedcode -m -s /bin/bash speedcode && \
    mkdir -p /workspace /tmp/speedcode && \
    chown -R speedcode:speedcode /workspace /tmp/speedcode

# ========================================================
# Target: API Gateway
# ========================================================
FROM alpine:3.19 AS api
RUN apk --no-cache add ca-certificates curl
COPY --from=builder /bin/speedcode-api /usr/local/bin/speedcode-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/speedcode-api"]

# ========================================================
# Target: Execution Worker Daemon
# ========================================================
FROM base-runtime AS worker
COPY --from=builder /bin/speedcode-worker /usr/local/bin/speedcode-worker
COPY --from=builder /bin/speedcode-engine /usr/local/bin/speedcode-engine

WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/speedcode-worker"]

# Default runner target
FROM base-runtime AS runner
WORKDIR /workspace
USER speedcode
CMD ["/bin/bash"]
