# ⚡ SpeedCode: Isolated Remote Code Execution Engine

> High-performance, isolated Linux execution engine kernel 
---
# 🏗 Architecture Overview

<img width="1536" height="1024" alt="ChatGPT Image Aug 30, 2026, 05_18_39 PM" src="https://github.com/user-attachments/assets/1dcdc40a-acba-4f3c-8c5e-c474dec3e893" />


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

