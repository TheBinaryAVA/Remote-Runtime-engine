# ⚡ SpeedCode: Isolated Remote Code Execution Engine

> High-performance, isolated Linux execution engine kernel built for **GDG VIT Chennai's Speed-Coding Event**.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Linux cgroups v2](https://img.shields.io/badge/Isolation-cgroups%20v2-orange)](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

---

## 📖 Table of Contents
- [Architecture Overview](#-architecture-overview)
- [Distributed Worker Architecture (Phase 2)](#-distributed-worker-architecture-phase-2)
- [System Requirements & Kernel Setup](#-system-requirements--kernel-setup)
- [Supported Languages](#-supported-languages)
- [Deterministic Metrics & Verdicts](#-deterministic-metrics--verdicts)
- [Security & Sandboxing Defenses](#-security--sandboxing-defenses)
- [Installation & Quick Start](#-installation--quick-start)
- [Docker Compose Deployment](#-docker-compose-deployment)
- [REST & WebSocket API](#-rest--websocket-api)
- [CLI Reference](#-cli-reference)
- [Malicious Test Payloads](#-malicious-test-payloads)
- [Testing](#-testing)
- [Roadmap](#-roadmap)

---

## 🏗 Architecture Overview

```
                      +-----------------------------------+
                      |   Client Application / Web UI     |
                      +-----------------+-----------------+
                                        |
                 +----------------------+----------------------+
                 | (REST POST /submissions)                   | (WebSocket /ws)
                 v                                             v
+-----------------------------------+       +-----------------------------------+
|      REST API Gateway             |       |    WebSocket Streaming Hub        |
|  - Rate Limiting & Backpressure   |       |  - Multiplexed Event Channels     |
|  - Request Validation             |       |  - Real-time JSON Frame Broadcast |
+-----------------+-----------------+       +-----------------+-----------------+
                  |                                           ^
                  v                                           | (Subscribe submission:id)
+-----------------------------------+       +-----------------+-----------------+
|  Distributed Task Queue (Redis)   |       |    Redis Pub/Sub Event Bus        |
|  - Atomic LPUSH / BRPOP           |       |  - QUEUED -> COMPILING -> RUNNING |
|  - Depth Monitoring               |       |  - TESTCASE_PASSED -> COMPLETED   |
+-----------------+-----------------+       +-----------------+-----------------+
                  |                                           ^
                  +---------------------+---------------------+
                                        | (Pull Job / Publish Events)
                                        v
                      +-----------------------------------+
                      |    Scalable Worker Pool Daemon    |
                      |  - Concurrent Goroutine Workers   |
                      |  - Warm-Pool Workspace Recycling  |
                      |  - One-Time Compilation Cache     |
                      |  - Crash & Panic Recovery Guards  |
                      +-----------------+-----------------+
                                        |
                 +----------------------+----------------------+
                 |                                             |
                 v                                             v
+-------------------------------+             +-------------------------------+
|    Native Linux cgroups v2    |             |    Containerized OCI Runner   |
|  - memory.max / memory.peak   |             |  - --read-only rootfs         |
|  - cpu.max / cpu.stat         |             |  - --network none             |
|  - pids.max (fork bomb def)   |             |  - --tmpfs ephemeral mounts   |
|  - unprivileged UID:GID       |             |  - --cap-drop ALL             |
|  - wall-clock SIGKILL timer   |             |  - --memory / --cpus          |
+-------------------------------+             +-------------------------------+
```

---

## ⚡ Distributed Worker Architecture (Phase 2)

Phase 2 scales the core engine into a high-concurrency event-driven cluster:
- **Asynchronous Task Ingestion**: Submissions are pushed to Redis queues, returning `202 Accepted` immediately with a WebSocket subscription URL.
- **Backpressure Protection**: Automatically protects the engine cluster by returning `429 Too Many Requests` (`Retry-After: 5`) when queue depth exceeds safety limits (`MaxQueueDepth = 500`).
- **Warm Compilation Cache**: For compiled languages (C++), the source is compiled **once** per submission and executed across all testcases in the batch, slashing latency.
- **WebSocket Streaming**: Live events (`QUEUED`, `COMPILING`, `RUNNING`, `TESTCASE_START`, `TESTCASE_PASSED`, `TESTCASE_FAILED`, `COMPLETED`, `FAILED`) stream directly to the browser.
- **Zero-Dependency Mode**: Automatic fallback to in-memory queues, event channels, and stores when Redis is not present.

---

## ⚙ System Requirements & Kernel Setup

### Native Linux Host
- **Linux Kernel**: Version 5.8+ recommended (with unified cgroups v2 enabled).
- **Filesystem**: Unified cgroup hierarchy mounted at `/sys/fs/cgroup`.
- **User Permissions**: Root permissions required for direct cgroups v2 controller creation.

#### Enabling Cgroups v2 on Ubuntu/Debian
```bash
stat -fc %T /sys/fs/cgroup
# Should output: cgroup2fs
```
If not enabled, update `/etc/default/grub`:
```
GRUB_CMDLINE_LINUX="systemd.unified_cgroup_hierarchy=1 cgroup_no_v1=all"
```
Run `sudo update-grub` and reboot.

---

## 💻 Supported Languages

| Language | Extension | Compiler / Interpreter | Flags |
| :--- | :--- | :--- | :--- |
| **C++** | `.cpp` | `g++` (GCC) | `-O3 -std=c++17 -Wall -Wextra -DONLINE_JUDGE -pipe` |
| **Python** | `.py` | `python3` | `-u` (unbuffered I/O), `-B` (suppress bytecode caching) |

---

## 📊 Deterministic Metrics & Verdicts

### Verdict Status Codes
- `ACCEPTED`: Solution compiled and passed all testcases within resource limits.
- `WRONG_ANSWER`: Solution ran successfully but stdout differed from expected output.
- `TIME_LIMIT_EXCEEDED` (TLE): Execution exceeded wall-clock timeout or CPU quota.
- `MEMORY_LIMIT_EXCEEDED` (MLE): Process breached `memory.max` and was killed by OOM.
- `COMPILATION_ERROR`: Failure during the compilation phase with diagnostic stderr.
- `RUNTIME_ERROR`: Process exited with non-zero exit code or fatal signal (`SIGSEGV`, `SIGFPE`).
- `OUTPUT_LIMIT_EXCEEDED`: Output exceeded maximum allowed stream buffer (1MB).
- `SYSTEM_ERROR`: Internal orchestration or host failure.

---

## 🛡 Security & Sandboxing Defenses

1. **Memory Isolation (`memory.max`)**: Hard memory ceiling. The Linux kernel guarantees no process or child in the cgroup exceeds this limit.
2. **CPU Throttling (`cpu.max`)**: Enforces CFS quota (e.g. `100000 100000` for 1 full core) preventing multi-threaded CPU starvation.
3. **Fork-Bomb Defense (`pids.max`)**: Restricts maximum processes/threads to `32`, rendering `fork()` exhaustion attacks harmless.
4. **Unprivileged Execution**: Code runs strictly under `nobody` or unprivileged UID `1001:1001` with `no_new_privs`.
5. **Network Disconnection**: Sockets and networking blocked (`--network none` in container mode).
6. **Disk & Output Exhaustion Caps**: Bounded stream readers cap output at `1MB` to prevent disk filling and memory buffer overflow.
7. **External Watchdog**: An asynchronous watchdog forcefully issues `SIGKILL` to the process group if wall-clock limits are exceeded.

---

## 🚀 Installation & Quick Start

### Build All Binaries
```bash
# Build CLI engine, API server, and Worker daemon
make build-all
```

### Launch Distributed Cluster with Docker Compose
```bash
docker-compose up -d --build
```
This spins up:
- **Redis 7** on port `6379`
- **SpeedCode REST & WebSocket API Gateway** on port `8080`
- **Worker Pool #1** (Concurrency 4)
- **Worker Pool #2** (Concurrency 4)

---

## 📡 REST & WebSocket API

Detailed API documentation is available in [API.md](file:///c:/Cloud%20Projects/API.md).

### Submit a Code Job
```bash
curl -X POST http://localhost:8080/api/v1/submissions \
  -H "Content-Type: application/json" \
  -d '{
    "language": "python3",
    "code": "a, b = map(int, input().split())\nprint(a + b)",
    "test_cases": [
      {"id": "tc-1", "input": "5 7\n", "expected_output": "12\n"},
      {"id": "tc-2", "input": "100 200\n", "expected_output": "300\n"}
    ]
  }'
```

**Response (`202 Accepted`):**
```json
{
  "submission_id": "sub-a1b2c3d4e5f6",
  "status": "QUEUED",
  "ws_url": "/api/v1/submissions/sub-a1b2c3d4e5f6/ws",
  "enqueued_at": "2026-08-30T10:45:00.000Z"
}
```

### Subscribe via WebSocket
```javascript
const ws = new WebSocket("ws://localhost:8080/api/v1/submissions/sub-a1b2c3d4e5f6/ws");
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log("Live Event:", data.status, data);
};
```

---

## 🛠 CLI Reference

```bash
# Standalone CLI execution
## 📖 Table of Contents
- [Architecture Overview](#-architecture-overview)
- [Distributed Worker Architecture (Phase 2)](#-distributed-worker-architecture-phase-2)
- [System Requirements & Kernel Setup](#-system-requirements--kernel-setup)
- [Supported Languages](#-supported-languages)
- [Deterministic Metrics & Verdicts](#-deterministic-metrics--verdicts)
- [Security & Sandboxing Defenses](#-security--sandboxing-defenses)
- [Installation & Quick Start](#-installation--quick-start)
- [Docker Compose Deployment](#-docker-compose-deployment)
- [REST & WebSocket API](#-rest--websocket-api)
- [CLI Reference](#-cli-reference)
- [Malicious Test Payloads](#-malicious-test-payloads)
- [Security Audit & Benchmarking Verification (Phase 3)](#-security-audit--benchmarking-verification-phase-3)
- [Prometheus Observability & Metrics](#-prometheus-observability--metrics)
- [Kubernetes Production Deployment](#-kubernetes-production-deployment)
- [Testing](#-testing)
- [Roadmap](#-roadmap)

---

## 🏗 Architecture Overview

```
                      +-----------------------------------+
                      |   Client Application / Web UI     |
                      +-----------------+-----------------+
                                        |
                 +----------------------+----------------------+
                 | (REST POST /submissions)                   | (WebSocket /ws)
                 v                                             v
+-----------------------------------+       +-----------------------------------+
|      REST API Gateway             |       |    WebSocket Streaming Hub        |
|  - Rate Limiting & Backpressure   |       |  - Multiplexed Event Channels     |
|  - Request Validation             |       |  - Real-time JSON Frame Broadcast |
+-----------------+-----------------+       +-----------------+-----------------+
                  |                                           ^
                  v                                           | (Subscribe submission:id)
+-----------------------------------+       +-----------------+-----------------+
|  Distributed Task Queue (Redis)   |       |    Redis Pub/Sub Event Bus        |
|  - Atomic LPUSH / BRPOP           |       |  - QUEUED -> COMPILING -> RUNNING |
|  - Depth Monitoring               |       |  - TESTCASE_PASSED -> COMPLETED   |
+-----------------+-----------------+       +-----------------+-----------------+
                  |                                           ^
                  +---------------------+---------------------+
                                        | (Pull Job / Publish Events)
                                        v
                      +-----------------------------------+
                      |    Scalable Worker Pool Daemon    |
                      |  - Concurrent Goroutine Workers   |
                      |  - Warm-Pool Workspace Recycling  |
                      |  - One-Time Compilation Cache     |
                      |  - Crash & Panic Recovery Guards  |
                      +-----------------+-----------------+
                                        |
                 +----------------------+----------------------+
                 |                                             |
                 v                                             v
+-------------------------------+             +-------------------------------+
|    Native Linux cgroups v2    |             |    Containerized OCI Runner   |
|  - memory.max / memory.peak   |             |  - --read-only rootfs         |
|  - cpu.max / cpu.stat         |             |  - --network none             |
|  - pids.max (fork bomb def)   |             |  - --tmpfs ephemeral mounts   |
|  - unprivileged UID:GID       |             |  - --cap-drop ALL             |
|  - wall-clock SIGKILL timer   |             |  - --memory / --cpus          |
+-------------------------------+             +-------------------------------+
```

---

## ⚡ Distributed Worker Architecture (Phase 2)

Phase 2 scales the core engine into a high-concurrency event-driven cluster:
- **Asynchronous Task Ingestion**: Submissions are pushed to Redis queues, returning `202 Accepted` immediately with a WebSocket subscription URL.
- **Backpressure Protection**: Automatically protects the engine cluster by returning `429 Too Many Requests` (`Retry-After: 5`) when queue depth exceeds safety limits (`MaxQueueDepth = 500`).
- **Warm Compilation Cache**: For compiled languages (C++), the source is compiled **once** per submission and executed across all testcases in the batch, slashing latency.
- **WebSocket Streaming**: Live events (`QUEUED`, `COMPILING`, `RUNNING`, `TESTCASE_START`, `TESTCASE_PASSED`, `TESTCASE_FAILED`, `COMPLETED`, `FAILED`) stream directly to the browser.
- **Zero-Dependency Mode**: Automatic fallback to in-memory queues, event channels, and stores when Redis is not present.

---

## ⚙ System Requirements & Kernel Setup

### Native Linux Host
- **Linux Kernel**: Version 5.8+ recommended (with unified cgroups v2 enabled).
- **Filesystem**: Unified cgroup hierarchy mounted at `/sys/fs/cgroup`.
- **User Permissions**: Root permissions required for direct cgroups v2 controller creation.

#### Enabling Cgroups v2 on Ubuntu/Debian
```bash
stat -fc %T /sys/fs/cgroup
# Should output: cgroup2fs
```
If not enabled, update `/etc/default/grub`:
```
GRUB_CMDLINE_LINUX="systemd.unified_cgroup_hierarchy=1 cgroup_no_v1=all"
```
Run `sudo update-grub` and reboot.

---

## 💻 Supported Languages

| Language | Extension | Compiler / Interpreter | Flags |
| :--- | :--- | :--- | :--- |
| **C++** | `.cpp` | `g++` (GCC) | `-O3 -std=c++17 -Wall -Wextra -DONLINE_JUDGE -pipe` |
| **Python** | `.py` | `python3` | `-u` (unbuffered I/O), `-B` (suppress bytecode caching) |

---

## 📊 Deterministic Metrics & Verdicts

### Verdict Status Codes
- `ACCEPTED`: Solution compiled and passed all testcases within resource limits.
- `WRONG_ANSWER`: Solution ran successfully but stdout differed from expected output.
- `TIME_LIMIT_EXCEEDED` (TLE): Execution exceeded wall-clock timeout or CPU quota.
- `MEMORY_LIMIT_EXCEEDED` (MLE): Process breached `memory.max` and was killed by OOM.
- `COMPILATION_ERROR`: Failure during the compilation phase with diagnostic stderr.
- `RUNTIME_ERROR`: Process exited with non-zero exit code or fatal signal (`SIGSEGV`, `SIGFPE`).
- `OUTPUT_LIMIT_EXCEEDED`: Output exceeded maximum allowed stream buffer (1MB).
- `SYSTEM_ERROR`: Internal orchestration or host failure.

---

## 🛡 Security & Sandboxing Defenses

1. **Memory Isolation (`memory.max`)**: Hard memory ceiling. The Linux kernel guarantees no process or child in the cgroup exceeds this limit.
2. **CPU Throttling (`cpu.max`)**: Enforces CFS quota (e.g. `100000 100000` for 1 full core) preventing multi-threaded CPU starvation.
3. **Fork-Bomb Defense (`pids.max`)**: Restricts maximum processes/threads to `32`, rendering `fork()` exhaustion attacks harmless.
4. **Unprivileged Execution**: Code runs strictly under `nobody` or unprivileged UID `1001:1001` with `no_new_privs`.
5. **Network Disconnection**: Sockets and networking blocked (`--network none` in container mode).
6. **Disk & Output Exhaustion Caps**: Bounded stream readers cap output at `1MB` to prevent disk filling and memory buffer overflow.
7. **External Watchdog**: An asynchronous watchdog forcefully issues `SIGKILL` to the process group if wall-clock limits are exceeded.

---

## 🚀 Installation & Quick Start

### Build All Binaries
```bash
# Build CLI engine, API server, and Worker daemon
make build-all
```

### Launch Distributed Cluster with Docker Compose
```bash
docker-compose up -d --build
```
This spins up:
- **Redis 7** on port `6379`
- **SpeedCode REST & WebSocket API Gateway** on port `8080`
- **Worker Pool #1** (Concurrency 4)
- **Worker Pool #2** (Concurrency 4)

---

## 📡 REST & WebSocket API

Detailed API documentation is available in [API.md](file:///c:/Cloud%20Projects/API.md).

### Submit a Code Job
```bash
curl -X POST http://localhost:8080/api/v1/submissions \
  -H "Content-Type: application/json" \
  -d '{
    "language": "python3",
    "code": "a, b = map(int, input().split())\nprint(a + b)",
    "test_cases": [
      {"id": "tc-1", "input": "5 7\n", "expected_output": "12\n"},
      {"id": "tc-2", "input": "100 200\n", "expected_output": "300\n"}
    ]
  }'
```

**Response (`202 Accepted`):**
```json
{
  "submission_id": "sub-a1b2c3d4e5f6",
  "status": "QUEUED",
  "ws_url": "/api/v1/submissions/sub-a1b2c3d4e5f6/ws",
  "enqueued_at": "2026-08-30T10:45:00.000Z"
}
```

### Subscribe via WebSocket
```javascript
const ws = new WebSocket("ws://localhost:8080/api/v1/submissions/sub-a1b2c3d4e5f6/ws");
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log("Live Event:", data.status, data);
};
```

---

## 🛠 CLI Reference

```bash
# Standalone CLI execution
./bin/speedcode-engine --file=solution.cpp --lang=cpp --input="10 20\n" --expected="30" --json
```

---

## 🛡 Security Audit & Benchmarking Verification (Phase 3)

Phase 3 delivers kernel-level exploit resilience, deterministic low-variance benchmarking, and continuous Prometheus observability.

### 1. Seccomp-BPF Syscall Filtering
Our custom [`seccomp-profile.json`](file:///c:/Cloud%20Projects/seccomp-profile.json) enforces an explicit syscall whitelist:
- **Kills Dangerous Syscalls**: `ptrace`, `reboot`, `mount`, `umount`, `pivot_root`, `chroot`, `kexec_load`, `bpf`, `setns`, `unshare`.
- **Denies Network Creation**: `socket`, `bind`, `connect`, `listen`, `accept`, `sendto`, `recvfrom` (`EPERM` / `ENETUNREACH`).
- **Required Linux Capabilities**: `CAP_SYS_ADMIN` and `CAP_SYS_RESOURCE` (for loading Seccomp filters, cgroups v2 controller manipulation, and CPU core pinning).

### 2. Deterministic CPU Pinning & Monotonic Timing
- **Hardware Isolation**: Worker routines allocate dedicated physical cores (`taskset` / `cpuset.cpus` / `sched_setaffinity`), preventing cache thrashing and OS scheduler migration noise.
- **Monotonic High-Resolution Timing**: Microsecond-accurate process telemetry recorded via `cpu.stat` (`user_usec` + `system_usec`) and monotonic timers (< 2% variance on identical runs).
- **RAM-Backed Scratch Space**: Code executes inside RAM-backed `tmpfs` mounts (`size=64m,noexec,nosuid,nodev`), eliminating disk I/O bottlenecks.

### 3. Automated Chaos Security Test Suite
The security audit suite tests real-world exploit payloads in `testdata/exploits/`:

| Exploit Payload | Attack Vector | Expected Defense |
| :--- | :--- | :--- |
| `testdata/exploits/fork_bomb.py` | Fork bomb process exhaustion | Blocked safely via cgroup `pids.max = 32` |
| `testdata/exploits/socket_attack.py` | Outbound TCP socket connection | Blocked by Seccomp / `CLONE_NEWNET` |
| `testdata/exploits/file_escape.py` | Unauthorized `/etc/shadow` access | Blocked by unprivileged UID `1001:1001` & read-only mounts |
| `testdata/exploits/syscall_exploit.cpp` | Unauthorized `ptrace` / `reboot` syscall | Terminated via Seccomp `SCMP_ACT_KILL_PROCESS` |
| `testdata/exploits/oom_attack.py` | Rapid physical RAM flood | Terminated by cgroup `memory.max` OOM killer |

To execute the chaos audit suite:
```bash
go test -v ./pkg/security/...
```

---

## 📊 Prometheus Observability & Metrics

Metrics are exposed on the API gateway at `GET /metrics`:

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `speedcode_submissions_total` | Counter | Total submissions by `language`, `verdict`, `sandbox_backend` |
| `speedcode_queue_depth` | Gauge | Current queue depth of pending jobs |
| `speedcode_execution_duration_seconds` | Histogram | Wall-clock execution duration distribution |
| `speedcode_cpu_time_seconds` | Histogram | CPU user + system execution duration |
| `speedcode_memory_usage_bytes` | Histogram | Peak physical RAM consumption in bytes |
| `speedcode_security_violations_total` | Counter | Security boundary violations by `violation_type` |
| `speedcode_active_workers` | Gauge | Number of active worker routines |

---

## ☸ Kubernetes Production Deployment

Kubernetes manifests are located in [`k8s/`](file:///c:/Cloud%20Projects/k8s/):
```bash
# 1. Apply Seccomp Profile ConfigMap
kubectl apply -f k8s/seccomp-configmap.yaml

# 2. Deploy Redis StatefulSet
kubectl apply -f k8s/redis-statefulset.yaml

# 3. Deploy API Gateway Deployment & Service
kubectl apply -f k8s/api-deployment.yaml

# 4. Deploy CPU-Pinned Execution Worker DaemonSet
kubectl apply -f k8s/worker-daemonset.yaml
```

---

## 🧪 Malicious Test Payloads

Included in `testdata/payloads/`:
- `infinite_loop/` -> `TIME_LIMIT_EXCEEDED`
- `memory_hog/` -> `MEMORY_LIMIT_EXCEEDED`
- `fork_bomb/` -> `RUNTIME_ERROR` / `pids.max` blocked
- `runtime_error/` -> `RUNTIME_ERROR`
- `compile_error/` -> `COMPILATION_ERROR`
- `accepted/` -> `ACCEPTED`

---

## 🔬 Testing

Run the complete Phase 1, Phase 2, and Phase 3 test suite:
```bash
make test
```

---

## 🗺 Roadmap

- [ ] **Phase 3: Expanded Language Support**: Java 21, Rust 1.78, Go 1.22, Node.js 20.
- [ ] **Phase 4: gRPC & REST Gateway**: Real-time WebSocket streaming of live execution states to the SpeedCode contest portal.
- [ ] **Phase 5: Seccomp & eBPF Sandboxing**: Syscall filtering (`seccomp-bpf`) to restrict kernel syscall attacks.

---

### Developed for GDG VIT Chennai
Created with ❤️ by the GDG VIT Chennai Backend & Systems Engineering Team.
