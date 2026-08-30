# ⚡ SpeedCode: Isolated Remote Code Execution Engine

> High-performance, isolated Linux execution engine kernel built for **GDG VIT Chennai's Speed-Coding Event**.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Linux cgroups v2](https://img.shields.io/badge/Isolation-cgroups%20v2-orange)](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

---

## 📖 Table of Contents
- [Architecture Overview](#-architecture-overview)
- [System Requirements & Kernel Setup](#-system-requirements--kernel-setup)
- [Supported Languages (Phase 1)](#-supported-languages-phase-1)
- [Deterministic Metrics & Verdicts](#-deterministic-metrics--verdicts)
- [Security & Sandboxing Defenses](#-security--sandboxing-defenses)
- [Installation & Quick Start](#-installation--quick-start)
- [CLI Reference](#-cli-reference)
- [Malicious Test Payloads](#-malicious-test-payloads)
- [Testing](#-testing)
- [Roadmap (Phase 2 & Beyond)](#-roadmap-phase-2--beyond)

---

## 🏗 Architecture Overview

```
                      +-----------------------------+
                      |   CLI / Queue Consumer API  |
                      +--------------+--------------+
                                     |
                                     v
                      +-----------------------------+
                      |   Execution Engine Kernel   |
                      +--------------+--------------+
                                     |
        +----------------------------+----------------------------+
        |                                                         |
        v                                                         v
+-------------------------+                             +-------------------------+
|  Language Handlers      |                             |   Sandbox Factory       |
|  - C++ (g++ C++17)      |                             |  - Native cgroup v2     |
|  - Python (Python 3.10+) |                             |  - Docker Container     |
+-------------------------+                             |  - Dev Fallback         |
                                                        +------------+------------+
                                                                     |
                                             +-----------------------+-----------------------+
                                             |                                               |
                                             v                                               v
                             +-------------------------------+               +-------------------------------+
                             |    Native Linux cgroups v2    |               |    Containerized OCI Runner   |
                             |  - memory.max / memory.peak   |               |  - --read-only rootfs         |
                             |  - cpu.max / cpu.stat         |               |  - --network none             |
                             |  - pids.max (fork bomb def)   |               |  - --tmpfs ephemeral mounts   |
                             |  - unprivileged UID:GID       |               |  - --cap-drop ALL             |
                             |  - wall-clock SIGKILL timer   |               |  - --memory / --cpus          |
                             +-------------------------------+               +-------------------------------+
                                             |                                               |
                                             +-----------------------+-----------------------+
                                                                     |
                                                                     v
                                                     +-------------------------------+
                                                     | Metric & Verdict Aggregator   |
                                                     | (Wall/CPU Time, Peak Mem, OOM)|
                                                     +-------------------------------+
```

---

## ⚙ System Requirements & Kernel Setup

### Native Linux Host
- **Linux Kernel**: Version 5.8+ recommended (with unified cgroups v2 enabled).
- **Filesystem**: Unified cgroup hierarchy mounted at `/sys/fs/cgroup`.
- **User Permissions**: Root permissions required to manage `/sys/fs/cgroup/speedcode` and switch unprivileged process credentials (`UID 1001: GID 1001`).

#### Enabling Cgroups v2 on Ubuntu/Debian
Ensure systemd boots with unified hierarchy by checking:
```bash
# Verify cgroups v2 mount
stat -fc %T /sys/fs/cgroup
# Output should be: cgroup2fs
```
If not enabled, add the following to `/etc/default/grub`:
```
GRUB_CMDLINE_LINUX="systemd.unified_cgroup_hierarchy=1 cgroup_no_v1=all"
```
Then run `sudo update-grub` and reboot.

---

## 💻 Supported Languages (Phase 1)

| Language | Extension | Compiler / Interpreter | Flags |
| :--- | :--- | :--- | :--- |
| **C++** | `.cpp` | `g++` (GCC) | `-O3 -std=c++17 -Wall -Wextra -DONLINE_JUDGE -pipe` |
| **Python** | `.py` | `python3` | `-u` (unbuffered I/O), `-B` (suppress bytecode caching) |

---

## 📊 Deterministic Metrics & Verdicts

### Verdict Status Codes
- `ACCEPTED`: Solution compiled, ran within all resource limits, and output matched expected solution.
- `WRONG_ANSWER`: Solution ran successfully but stdout differed from expected testcase output.
- `TIME_LIMIT_EXCEEDED` (TLE): Execution exceeded wall-clock timeout or CPU quota limit.
- `MEMORY_LIMIT_EXCEEDED` (MLE): Process breached `memory.max` and was terminated by the kernel OOM-killer.
- `COMPILATION_ERROR`: Failure during the compilation phase with diagnostic stderr output.
- `RUNTIME_ERROR`: Process exited with non-zero exit code or fatal signal (e.g. `SIGSEGV`, `SIGFPE`, division by zero).
- `OUTPUT_LIMIT_EXCEEDED`: Program exceeded the maximum allowed stdout/stderr buffer byte limit (e.g. 1MB).
- `SYSTEM_ERROR`: Host filesystem or internal orchestration failure.

### Measured Metrics
- **Wall Time**: Real-time duration measured with microsecond precision.
- **CPU Time**: Total CPU time (User space + Kernel space) extracted from `cpu.stat` or process `rusage`.
- **Peak Memory**: Maximum memory high-water mark extracted directly from `memory.peak` in bytes.
- **Exit Code**: Exact Linux process exit code or signal termination code.

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

### Build Binary
```bash
# Build engine CLI
go build -o speedcode-engine cmd/engine/main.go
```

### Run Sample Executions

#### 1. Python Accepted Test
```bash
./speedcode-engine \
  --file=testdata/payloads/accepted/solution.py \
  --lang=python3 \
  --input="10 25\n" \
  --expected="35"
```

#### 2. JSON Output Mode
```bash
./speedcode-engine \
  --file=testdata/payloads/accepted/solution.py \
  --lang=python3 \
  --input="15 27\n" \
  --expected="42" \
  --json
```

**Output:**
```json
{
  "id": "exec-e102e75359cf5622",
  "verdict": "ACCEPTED",
  "exit_code": 0,
  "stdout": "42\n",
  "stderr": "",
  "wall_time_ms": 255.83,
  "cpu_time_ms": 230.25,
  "peak_memory_kb": 1024,
  "peak_memory_mb": 1.00,
  "oom_killed": false,
  "sandbox_backend": "native_cgroupv2",
  "executed_at": "2026-08-30T10:44:36.326Z"
}
```

---

## 🛠 CLI Reference

```
Usage of speedcode-engine:
  -file string
        Path to source code file (e.g. solution.cpp, solution.py)
  -code string
        Inline source code string
  -lang string
        Programming language (cpp, python3)
  -input string
        Standard input string or @filepath (e.g. @testcase.in)
  -expected string
        Expected output string to verify ACCEPTED vs WRONG_ANSWER
  -time-limit-ms int
        Wall-clock time limit in milliseconds (default 2000)
  -memory-limit-mb int
        Memory limit in Megabytes (default 128)
  -cpu-quota float
        CPU core quota (default 1.0)
  -pids-limit int
        Maximum PIDs / threads limit (default 32)
  -sandbox string
        Sandbox backend: auto, native, docker, dev_process (default "auto")
  -json
        Output result as structured JSON
```

---

## 🧪 Malicious Test Payloads

Included in `testdata/payloads/`:

| Payload | Description | Expected Verdict |
| :--- | :--- | :--- |
| `infinite_loop/loop.py` | Infinite `while True` loop | `TIME_LIMIT_EXCEEDED` |
| `memory_hog/oom.py` | Rapid multi-megabyte physical allocations | `MEMORY_LIMIT_EXCEEDED` |
| `fork_bomb/bomb.py` | Recursive process spawning (`os.fork()`) | `RUNTIME_ERROR` / `pids.max` blocked |
| `runtime_error/error.py` | Division by zero (`ZeroDivisionError`) | `RUNTIME_ERROR` |
| `compile_error/bad.cpp` | Syntactically invalid C++ program | `COMPILATION_ERROR` |
| `accepted/solution.py` | Standard competitive programming solution | `ACCEPTED` |

---

## 🔬 Testing

Run the full test suite with race detector:
```bash
go test -v -race ./...
```

---

## 🗺 Roadmap (Phase 2 & Beyond)

- [ ] **Phase 2: Asynchronous Distributed Queue**: Redis/RabbitMQ job ingestion workers with horizontal scaling.
- [ ] **Phase 3: Expanded Language Support**: Java 21, Rust 1.78, Go 1.22, Node.js 20.
- [ ] **Phase 4: gRPC & REST Gateway**: Real-time WebSocket streaming of live execution states to the SpeedCode contest portal.
- [ ] **Phase 5: Seccomp & eBPF Sandboxing**: Syscall filtering (`seccomp-bpf`) to restrict kernel syscall attacks.

---

### Developed for GDG VIT Chennai

