# 🚀 SpeedCode Remote Code Execution Engine — Definitive Reproduction & Setup Guide

> **GDG VIT Chennai Speed-Coding Platform**  
> Complete single-command instructions to clone, build, execute, and verify the Remote Code Execution platform end-to-end.

---

## 📋 Table of Contents
1. [Prerequisites & System Requirements](#1-prerequisites--system-requirements)
2. [Single-Command Start & Automated Verification](#2-single-command-start--automated-verification)
3. [Manual Code Submission & WebSocket Streaming](#3-manual-code-submission--websocket-streaming)
4. [Docker Compose & Kubernetes Deployment](#4-docker-compose--kubernetes-deployment)
5. [Automated Diagnostic Suite (`verify.py`)](#5-automated-diagnostic-suite-verifypy)
6. [Prometheus Monitoring & Observability](#6-prometheus-monitoring--observability)
7. [Troubleshooting Guide](#7-troubleshooting-guide)

---

## 1. Prerequisites & System Requirements

| Component | Minimum Requirement | Recommended | Notes |
| :--- | :--- | :--- | :--- |
| **Operating System** | Linux (Kernel 5.8+) / WSL2 / macOS | Ubuntu 22.04 LTS | Unified cgroups v2 enabled |
| **Go Toolchain** | Go 1.22+ | Go 1.24+ | Required for building engine binaries |
| **Python** | Python 3.8+ | Python 3.10+ | Used for verification suite & runner |
| **C++ Compiler** | `g++` (GCC 9+) | `g++` (GCC 11+) | Required for C++ submissions |
| **Docker (Optional)** | Docker 20.10+ | Docker Engine 24+ | For containerized cluster deployment |
| **Redis (Optional)** | Redis 6.2+ | Redis 7+ Alpine | Required for distributed multi-node mode |

### Linux Kernel Capabilities
When running native cgroup v2 controller manipulation and Seccomp-BPF filters on bare-metal Linux, the host user requires `root` or the following Linux capabilities:
- `CAP_SYS_ADMIN`: For configuring process namespaces (`CLONE_NEWNET`) and Seccomp filters.
- `CAP_SYS_RESOURCE`: For setting memory limits and CFS CPU quotas.

---

## 2. Single-Command Start & Automated Verification

Clone the repository and run the master setup script:

```bash
# 1. Clone repository
git clone https://github.com/TheBinaryAVA/Remote-Runtime-engine.git
cd Remote-Runtime-engine

# 2. Run single-command master setup & verification
chmod +x run.sh
./run.sh
```

### What `run.sh` Does Automatically:
1. **Kernel Check**: Validates unified cgroup v2 mounts, Docker daemon status, and Go/Python3 toolchains.
2. **Build Stage**: Compiles `speedcode-engine`, `speedcode-api`, and `speedcode-worker` into `bin/`.
3. **Platform Launch**: Spins up the API Gateway and Worker Pool.
4. **Diagnostic Suite**: Executes `verify.py` against all 8 diagnostic categories.
5. **Output**: Prints an ANSI color summary scorecard and displays live service URLs.

---

## 3. Manual Code Submission & WebSocket Streaming

### Step 1: Submit Code via REST API
Submit a Python submission with testcases:

```bash
curl -X POST http://localhost:8080/api/v1/submissions \
  -H "Content-Type: application/json" \
  -d '{
    "language": "python3",
    "code": "a, b = map(int, input().split())\nprint(a + b)",
    "time_limit_ms": 2000,
    "memory_limit_mb": 128,
    "test_cases": [
      {"id": "tc-1", "input": "10 25\n", "expected_output": "35\n"},
      {"id": "tc-2", "input": "100 250\n", "expected_output": "350\n"}
    ]
  }'
```

**Response (`202 Accepted`):**
```json
{
  "submission_id": "sub-7b2c9a1d4e8f",
  "status": "QUEUED",
  "ws_url": "/api/v1/submissions/sub-7b2c9a1d4e8f/ws",
  "enqueued_at": "2026-08-30T12:00:00Z"
}
```

### Step 2: Stream Live WebSocket Updates
Connect using `wscat` or JavaScript to receive live status frames:

```bash
# Using wscat CLI
npx wscat -c ws://localhost:8080/api/v1/submissions/sub-7b2c9a1d4e8f/ws
```

**Streamed WebSocket Frames:**
```json
{"submission_id":"sub-7b2c9a1d4e8f","status":"QUEUED","timestamp":"2026-08-30T12:00:00.100Z"}
{"submission_id":"sub-7b2c9a1d4e8f","status":"RUNNING","total_test_cases":2,"timestamp":"2026-08-30T12:00:00.200Z"}
{"submission_id":"sub-7b2c9a1d4e8f","status":"TESTCASE_PASSED","current_test_case":1,"total_test_cases":2,"wall_time_ms":18.4,"peak_memory_mb":1.2}
{"submission_id":"sub-7b2c9a1d4e8f","status":"TESTCASE_PASSED","current_test_case":2,"total_test_cases":2,"wall_time_ms":19.1,"peak_memory_mb":1.2}
{"submission_id":"sub-7b2c9a1d4e8f","status":"COMPLETED","verdict":"ACCEPTED","total_test_cases":2,"wall_time_ms":37.5,"peak_memory_mb":1.2}
```

### Step 3: Fetch Final Results via REST
```bash
curl -s http://localhost:8080/api/v1/submissions/sub-7b2c9a1d4e8f | jq .
```

---

## 4. Docker Compose & Kubernetes Deployment

### Launch Production Cluster via Docker Compose
```bash
# Spins up Redis 7, API Gateway, 2 CPU-Pinned Workers, and Prometheus Server
docker-compose -f docker-compose.prod.yml up -d --build
```

### Deploy to Kubernetes
```bash
# Apply Seccomp profile ConfigMap
kubectl apply -f k8s/seccomp-configmap.yaml

# Deploy Redis StatefulSet
kubectl apply -f k8s/redis-statefulset.yaml

# Deploy API Gateway Deployment & Service
kubectl apply -f k8s/api-deployment.yaml

# Deploy CPU-Pinned Worker DaemonSet
kubectl apply -f k8s/worker-daemonset.yaml
```

---

## 5. Automated Diagnostic Suite (`verify.py`)

Run the standalone end-to-end diagnostic verification suite:

```bash
python3 verify.py
```

### Test Matrix Executed:
1. **Health & Telemetry**: Asserts `/api/v1/health` responds with `healthy` and tracks queue depth.
2. **Standard Python Execution**: Verifies `ACCEPTED` verdict, correct stdout, and timing accuracy.
3. **Standard C++ Compilation & Run**: Verifies g++ compilation and execution.
4. **Time Limit Guard (TLE)**: Asserts infinite loop terminates at configured time limit.
5. **Memory Limit Guard (MLE / OOM)**: Asserts memory allocation flood is cleanly terminated.
6. **Fork Bomb Process Ceiling (`pids.max`)**: Verifies process exhaustion attempts are blocked.
7. **Filesystem Isolation**: Asserts host rootfs files (`/etc/shadow`) are inaccessible.
8. **20-Client Concurrent Burst**: Simulates 20 simultaneous submissions, asserting 0 dropped jobs.

---

## 6. Prometheus Monitoring & Observability

Access Prometheus metrics at:
```
http://localhost:8080/metrics
```

Key monitored metrics:
- `speedcode_submissions_total{language, verdict, sandbox_backend}`
- `speedcode_queue_depth`
- `speedcode_execution_duration_seconds` (Histogram)
- `speedcode_cpu_time_seconds` (Histogram)
- `speedcode_memory_usage_bytes` (Histogram)
- `speedcode_security_violations_total{violation_type}`
- `speedcode_active_workers`

---

## 7. Troubleshooting Guide

### 1. `cgroup2fs` Not Mounted / Permission Denied
- **Issue**: `failed to create cgroup: permission denied`.
- **Solution**: Ensure your Linux kernel is booted with unified cgroups v2. Check `stat -fc %T /sys/fs/cgroup`. If running natively, ensure the engine runs with `sudo` or runs inside Docker container with `--security-opt no-new-privileges:true`.

### 2. Docker Socket Permission Denied
- **Issue**: `Got permission denied while trying to connect to the Docker daemon socket`.
- **Solution**: Add your user to the `docker` group:
  ```bash
  sudo usermod -aG docker $USER
  newgrp docker
  ```

### 3. Port Already in Use (`8080` or `6379`)
- **Issue**: `bind: address already in use`.
- **Solution**: Set a custom port when launching the API gateway:
  ```bash
  ./bin/speedcode-api --port=8081
  SPEEDCODE_API_URL=http://localhost:8081 python3 verify.py
  ```

---

### Developed for GDG VIT Chennai
Created with ❤️ by the GDG VIT Chennai Backend & Systems Engineering Team.
