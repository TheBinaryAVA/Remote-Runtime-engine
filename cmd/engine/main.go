package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/engine"
	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/models"
)

func main() {
	filePath := flag.String("file", "", "Path to source code file")
	codeString := flag.String("code", "", "Inline source code string")
	language := flag.String("lang", "", "Programming language (cpp, python3)")
	stdin := flag.String("input", "", "Standard input string or @filepath")
	expected := flag.String("expected", "", "Expected output string")
	timeLimitMs := flag.Int64("time-limit-ms", models.DefaultTimeLimitMs, "Time limit in milliseconds")
	memoryLimitMB := flag.Int64("memory-limit-mb", models.DefaultMemoryLimitMB, "Memory limit in Megabytes")
	cpuQuota := flag.Float64("cpu-quota", models.DefaultCpuQuota, "CPU core quota (e.g. 1.0)")
	pidsLimit := flag.Int64("pids-limit", models.DefaultPidsLimit, "Maximum PIDs/threads")
	sandboxBackend := flag.String("sandbox", "auto", "Sandbox backend: auto, native, docker, dev_process")
	rawJSON := flag.Bool("json", false, "Output pure JSON")

	flag.Parse()

	// Validate code input
	var code string
	if *filePath != "" {
		data, err := os.ReadFile(*filePath)
		if err != nil {
			fatalError("failed to read file: %v", err)
		}
		code = string(data)

		// Auto-detect language from extension if not specified
		if *language == "" {
			ext := filepath.Ext(*filePath)
			switch ext {
			case ".cpp", ".cc", ".cxx":
				*language = "cpp"
			case ".py":
				*language = "python3"
			}
		}
	} else if *codeString != "" {
		code = *codeString
	} else {
		fatalError("either --file or --code must be specified. Run with -h for help.")
	}

	if *language == "" {
		fatalError("language must be specified via --lang (e.g. cpp, python3)")
	}

	// Parse stdin if it's a file reference starting with @
	stdinValue := *stdin
	if strings.HasPrefix(stdinValue, "@") {
		data, err := os.ReadFile(stdinValue[1:])
		if err != nil {
			fatalError("failed to read stdin file: %v", err)
		}
		stdinValue = string(data)
	}

	req := &models.ExecutionRequest{
		Language:       *language,
		Code:           code,
		Stdin:          stdinValue,
		ExpectedOutput: *expected,
		TimeLimitMs:    *timeLimitMs,
		MemoryLimitMB:  *memoryLimitMB,
		CpuQuota:       *cpuQuota,
		PidsLimit:      *pidsLimit,
		SandboxType:    *sandboxBackend,
	}

	eng := engine.New(*sandboxBackend)
	result, err := eng.Run(context.Background(), req)
	if err != nil {
		fatalError("engine execution failed: %v", err)
	}

	if *rawJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fatalError("failed to encode result JSON: %v", err)
		}
		return
	}

	printHumanReadable(result)
}

func printHumanReadable(res *models.ExecutionResult) {
	fmt.Println("==================================================")
	fmt.Printf("🚀 GDG VIT Chennai Remote Runtime Engine (Phase 1)\n")
	fmt.Println("==================================================")
	fmt.Printf("Submission ID  : %s\n", res.ID)
	fmt.Printf("Sandbox Backend: %s\n", res.SandboxBackend)
	fmt.Printf("Verdict        : %s\n", formatVerdict(res.Verdict))
	fmt.Printf("Exit Code      : %d\n", res.ExitCode)
	fmt.Printf("Wall Time      : %.2f ms\n", res.WallTimeMs)
	fmt.Printf("CPU Time       : %.2f ms\n", res.CpuTimeMs)
	fmt.Printf("Peak Memory    : %d KB (%.2f MB)\n", res.PeakMemoryKB, res.PeakMemoryMB)
	fmt.Printf("OOM Killed     : %t\n", res.OOMKilled)

	if res.Compilation != nil && !res.Compilation.Success {
		fmt.Printf("\n--- Compilation Error (Exit %d in %.2fms) ---\n%s\n",
			res.Compilation.ExitCode, res.Compilation.TimeMs, res.Compilation.Stderr)
	}

	if res.Stdout != "" {
		fmt.Printf("\n--- Program Stdout ---\n%s\n", res.Stdout)
	}

	if res.Stderr != "" && res.Verdict != models.VerdictCompilationError {
		fmt.Printf("\n--- Program Stderr ---\n%s\n", res.Stderr)
	}

	if res.ErrorDetails != "" {
		fmt.Printf("\n--- Error Details ---\n%s\n", res.ErrorDetails)
	}
	fmt.Println("==================================================")
}

func formatVerdict(v models.Verdict) string {
	switch v {
	case models.VerdictAccepted:
		return fmt.Sprintf("\033[32m[ ACCEPTED ]\033[0m")
	case models.VerdictTimeLimitExceeded:
		return fmt.Sprintf("\033[33m[ TIME_LIMIT_EXCEEDED ]\033[0m")
	case models.VerdictMemoryLimitExceeded:
		return fmt.Sprintf("\033[35m[ MEMORY_LIMIT_EXCEEDED ]\033[0m")
	case models.VerdictCompilationError:
		return fmt.Sprintf("\033[31m[ COMPILATION_ERROR ]\033[0m")
	case models.VerdictRuntimeError:
		return fmt.Sprintf("\033[31m[ RUNTIME_ERROR ]\033[0m")
	case models.VerdictWrongAnswer:
		return fmt.Sprintf("\033[31m[ WRONG_ANSWER ]\033[0m")
	default:
		return fmt.Sprintf("[%s]", v)
	}
}

func fatalError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
