package languages_test

import (
	"testing"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/languages"
)

func TestLanguageRegistry(t *testing.T) {
	cpp, err := languages.Get("cpp")
	if err != nil {
		t.Fatalf("expected cpp handler, got error: %v", err)
	}
	if cpp.Name() != "cpp" || !cpp.NeedsCompilation() {
		t.Errorf("cpp handler invalid properties")
	}

	py, err := languages.Get("python3")
	if err != nil {
		t.Fatalf("expected python3 handler, got error: %v", err)
	}
	if py.Name() != "python3" || py.NeedsCompilation() {
		t.Errorf("python handler invalid properties")
	}

	// Test aliases
	if _, err := languages.Get("c++"); err != nil {
		t.Errorf("expected c++ alias to work")
	}
	if _, err := languages.Get("python"); err != nil {
		t.Errorf("expected python alias to work")
	}

	// Test unsupported
	if _, err := languages.Get("brainfuck"); err == nil {
		t.Errorf("expected error for unsupported language")
	}
}
