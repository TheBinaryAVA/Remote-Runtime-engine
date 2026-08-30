package languages

import (
	"fmt"
	"strings"
	"sync"
)

// LanguageHandler defines the contract for compiling and running supported programming languages.
type LanguageHandler interface {
	// Name returns the canonical name of the language (e.g., "cpp", "python3").
	Name() string

	// SourceFilename returns the standardized source file name (e.g., "solution.cpp", "solution.py").
	SourceFilename() string

	// BinaryFilename returns the executable binary name (or empty if interpreted).
	BinaryFilename() string

	// NeedsCompilation returns true if the language requires an ahead-of-time compilation phase.
	NeedsCompilation() bool

	// CompileCommand returns the compiler executable and argument slice.
	CompileCommand(sourcePath, binaryPath string) (string, []string)

	// RunCommand returns the execution command and arguments for the prepared binary or script.
	RunCommand(targetPath string) (string, []string)
}

var (
	registryMu sync.RWMutex
	registry   = make(map[string]LanguageHandler)
)

func init() {
	Register(&CppHandler{})
	Register(&PythonHandler{})
}

// Register adds a language handler to the global registry.
func Register(handler LanguageHandler) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[strings.ToLower(handler.Name())] = handler
}

// Get returns the language handler for a given language identifier.
func Get(lang string) (LanguageHandler, error) {
	normalized := strings.ToLower(strings.TrimSpace(lang))
	if normalized == "python" || normalized == "py" {
		normalized = "python3"
	}
	if normalized == "c++" || normalized == "g++" {
		normalized = "cpp"
	}

	registryMu.RLock()
	defer registryMu.RUnlock()

	handler, exists := registry[normalized]
	if !exists {
		return nil, fmt.Errorf("unsupported language: '%s' (supported: cpp, python3)", lang)
	}
	return handler, nil
}

// SupportedLanguages returns a list of all registered language names.
func SupportedLanguages() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	return keys
}
