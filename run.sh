#!/usr/bin/env bash
# ==============================================================================
# ⚡ SpeedCode Remote Code Execution Engine - Master Setup & Diagnostic Script
# GDG VIT Chennai Speed-Coding Platform
# ==============================================================================

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "\n${CYAN}${BOLD}======================================================================"
echo -e "🚀 SpeedCode Remote Code Execution Engine - Master Setup & Verification"
echo -e "======================================================================${NC}\n"

# -----------------------------------------------------------------------------
# 1. Environment & Kernel Checks
# -----------------------------------------------------------------------------
echo -e "${BOLD}[1/4] Inspecting Host Kernel & Isolation Capabilities...${NC}"

# Check cgroups v2
if [ -d "/sys/fs/cgroup" ]; then
    CG_TYPE=$(stat -fc %T /sys/fs/cgroup 2>/dev/null || echo "unknown")
    if [ "$CG_TYPE" = "cgroup2fs" ]; then
        echo -e "  ${GREEN}✓ Linux Unified cgroups v2 active (${CG_TYPE})${NC}"
    else
        echo -e "  ${YELLOW}ℹ cgroups mount detected (${CG_TYPE}); container sandbox will be used${NC}"
    fi
else
    echo -e "  ${YELLOW}ℹ Running in non-Linux host environment; container / dev process sandbox active${NC}"
fi

# Check Docker availability
if command -v docker &> /dev/null; then
    if docker info &> /dev/null; then
        echo -e "  ${GREEN}✓ Docker daemon active & reachable${NC}"
        HAS_DOCKER=true
    else
        echo -e "  ${YELLOW}ℹ Docker CLI installed, but daemon is unreachable${NC}"
        HAS_DOCKER=false
    fi
else
    echo -e "  ${YELLOW}ℹ Docker not installed; running in native / standalone mode${NC}"
    HAS_DOCKER=false
fi

# Check Go toolchain
if command -v go &> /dev/null; then
    GO_VER=$(go version | awk '{print $3}')
    echo -e "  ${GREEN}✓ Go toolchain detected (${GO_VER})${NC}"
else
    echo -e "  ${RED}✗ Go toolchain not found. Please install Go 1.22+${NC}"
    exit 1
fi

# Check Python 3
if command -v python3 &> /dev/null; then
    PY_VER=$(python3 --version)
    echo -e "  ${GREEN}✓ Python 3 runtime detected (${PY_VER})${NC}"
elif command -v python &> /dev/null; then
    PY_VER=$(python --version)
    echo -e "  ${GREEN}✓ Python runtime detected (${PY_VER})${NC}"
else
    echo -e "  ${RED}✗ Python 3 not found for running the verification suite.${NC}"
    exit 1
fi

# -----------------------------------------------------------------------------
# 2. Build Engine & Service Binaries
# -----------------------------------------------------------------------------
echo -e "\n${BOLD}[2/4] Building Service Binaries (Engine, API Gateway, Worker Daemon)...${NC}"
mkdir -p bin

go build -ldflags="-s -w" -o bin/speedcode-engine cmd/engine/main.go
echo -e "  ${GREEN}✓ Compiled bin/speedcode-engine${NC}"

go build -ldflags="-s -w" -o bin/speedcode-api cmd/api/main.go
echo -e "  ${GREEN}✓ Compiled bin/speedcode-api${NC}"

go build -ldflags="-s -w" -o bin/speedcode-worker cmd/worker/main.go
echo -e "  ${GREEN}✓ Compiled bin/speedcode-worker${NC}"

# -----------------------------------------------------------------------------
# 3. Launching Engine Infrastructure
# -----------------------------------------------------------------------------
echo -e "\n${BOLD}[3/4] Launching Platform Services...${NC}"

API_PID=""
WORKER_PID=""

cleanup() {
    echo -e "\n${YELLOW}[*] Shutting down background services...${NC}"
    if [ -n "$API_PID" ]; then kill $API_PID 2>/dev/null || true; fi
    if [ -n "$WORKER_PID" ]; then kill $WORKER_PID 2>/dev/null || true; fi
    echo -e "${GREEN}✓ All services stopped cleanly.${NC}"
}
trap cleanup EXIT INT TERM

if [ "$HAS_DOCKER" = true ] && [ "$1" = "--docker" ]; then
    echo -e "  [*] Launching Docker Compose cluster (Redis, API, Workers)..."
    docker-compose up -d --build
    echo -e "  ${GREEN}✓ Docker Compose cluster running.${NC}"
else
    echo -e "  [*] Starting API Gateway & Worker Daemon in high-throughput standalone mode..."
    ./bin/speedcode-api --port=8080 --max-queue-depth=1000 &
    API_PID=$!

    ./bin/speedcode-worker --concurrency=4 --worker-id=worker-standalone-1 &
    WORKER_PID=$!
fi

# Wait for API health
echo -e "  [*] Waiting for API Gateway readiness on http://localhost:8080/api/v1/health ..."
READY=false
for i in {1..30}; do
    if curl -s http://localhost:8080/api/v1/health | grep -q "healthy"; then
        READY=true
        break
    fi
    sleep 0.2
done

if [ "$READY" = false ]; then
    echo -e "  ${RED}✗ API Gateway failed to become ready in time.${NC}"
    exit 1
fi
echo -e "  ${GREEN}✓ API Gateway is healthy and accepting traffic!${NC}"

# -----------------------------------------------------------------------------
# 4. Execute End-to-End Diagnostic Suite
# -----------------------------------------------------------------------------
echo -e "\n${BOLD}[4/4] Running End-to-End Automated Diagnostic & Verification Suite...${NC}"
if command -v python3 &> /dev/null; then
    python3 verify.py
else
    python verify.py
fi

echo -e "${CYAN}${BOLD}======================================================================"
echo -e "📡 Platform Access & Service Endpoints"
echo -e "======================================================================${NC}"
echo -e "  • REST API Base URL      : ${BOLD}http://localhost:8080/api/v1${NC}"
echo -e "  • WebSocket Gateway URL  : ${BOLD}ws://localhost:8080/api/v1/submissions/:id/ws${NC}"
echo -e "  • Prometheus Metrics     : ${BOLD}http://localhost:8080/metrics${NC}"
echo -e "  • Healthcheck Endpoint   : ${BOLD}http://localhost:8080/api/v1/health${NC}"
echo -e "${CYAN}======================================================================${NC}\n"
