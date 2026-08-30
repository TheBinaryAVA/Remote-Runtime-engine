package security

import (
	"encoding/json"
	"os"
)

// SeccompProfile defines the structure for OCI/Docker and Kubernetes Seccomp-BPF profiles.
type SeccompProfile struct {
	DefaultAction string        `json:"defaultAction"`
	Architectures []string      `json:"architectures"`
	Syscalls      []SyscallRule `json:"syscalls"`
}

// SyscallRule defines an action and matched syscall names.
type SyscallRule struct {
	Names  []string `json:"names"`
	Action string   `json:"action"`
	Args   []Arg    `json:"args,omitempty"`
}

// Arg defines argument filters for specific syscall parameters.
type Arg struct {
	Index    int    `json:"index"`
	Value    uint64 `json:"value"`
	ValueTwo uint64 `json:"valueTwo,omitempty"`
	Op       string `json:"op"`
}

// DefaultSeccompProfile returns a hardened Seccomp-BPF profile designed for competitive programming runtimes.
// It explicitly whitelists safe I/O, memory, and runtime syscalls while blocking dangerous host-level operations.
func DefaultSeccompProfile() *SeccompProfile {
	return &SeccompProfile{
		DefaultAction: "SCMP_ACT_ERRNO", // Block any unlisted syscall with EPERM
		Architectures: []string{
			"SCMP_ARCH_X86_64",
			"SCMP_ARCH_X86",
			"SCMP_ARCH_AARCH64",
		},
		Syscalls: []SyscallRule{
			// Safe I/O, File System (Read-Only/Ephemeral), and Basic Process Operations
			{
				Names: []string{
					"read",
					"write",
					"open",
					"openat",
					"close",
					"fstat",
					"stat",
					"lstat",
					"lseek",
					"mmap",
					"mprotect",
					"munmap",
					"brk",
					"rt_sigaction",
					"rt_sigprocmask",
					"rt_sigreturn",
					"ioctl",
					"pread64",
					"pwrite64",
					"readv",
					"writev",
					"access",
					"faccessat",
					"faccessat2",
					"pipe",
					"pipe2",
					"select",
					"pselect6",
					"poll",
					"ppoll",
					"nanosleep",
					"clock_nanosleep",
					"clock_gettime",
					"clock_getres",
					"gettimeofday",
					"getpid",
					"getppid",
					"gettid",
					"getuid",
					"getgid",
					"geteuid",
					"getegid",
					"exit",
					"exit_group",
					"futex",
					"sched_yield",
					"sched_getaffinity",
					"arch_prctl",
					"set_tid_address",
					"set_robust_list",
					"get_robust_list",
					"rseq",
					"prlimit64",
					"getrandom",
					"sysinfo",
					"uname",
					"getcwd",
					"readlink",
					"readlinkat",
					"fcntl",
					"dup",
					"dup2",
					"dup3",
					"sigaltstack",
					"epoll_create",
					"epoll_create1",
					"epoll_ctl",
					"epoll_wait",
					"epoll_pwait",
					"eventfd",
					"eventfd2",
					"clone",
					"clone3",
					"wait4",
					"waitid",
					"execve",
				},
				Action: "SCMP_ACT_ALLOW",
			},
			// Explicitly Kill on Dangerous Syscalls (Audit flagged)
			{
				Names: []string{
					"ptrace",
					"reboot",
					"kexec_load",
					"kexec_file_load",
					"bpf",
					"setns",
					"unshare",
					"mount",
					"umount",
					"umount2",
					"pivot_root",
					"chroot",
					"syslog",
					"init_module",
					"finit_module",
					"delete_module",
					"iopl",
					"ioperm",
					"swapon",
					"swapoff",
				},
				Action: "SCMP_ACT_KILL_PROCESS",
			},
			// Explicitly Deny Network Syscalls (No outbound/inbound connections)
			{
				Names: []string{
					"socket",
					"bind",
					"connect",
					"listen",
					"accept",
					"accept4",
					"sendto",
					"recvfrom",
					"sendmsg",
					"recvmsg",
					"shutdown",
					"getsockopt",
					"setsockopt",
				},
				Action: "SCMP_ACT_ERRNO",
			},
		},
	}
}

// WriteProfileToFile serializes the default profile to the given filepath.
func WriteProfileToFile(path string) error {
	profile := DefaultSeccompProfile()
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
