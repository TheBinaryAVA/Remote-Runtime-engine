package sandbox_test

import (
	"testing"

	"github.com/TheBinaryAVA/Remote-Runtime-engine/pkg/sandbox"
)

func TestBoundedBuffer_WithinLimit(t *testing.T) {
	buf := sandbox.NewBoundedBuffer(100)
	n, err := buf.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != 11 {
		t.Fatalf("expected 11 bytes written, got %d", n)
	}
	if buf.Exceeded() {
		t.Fatalf("expected exceeded to be false")
	}
	if buf.String() != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", buf.String())
	}
}

func TestBoundedBuffer_ExceedsLimit(t *testing.T) {
	buf := sandbox.NewBoundedBuffer(10)
	n, err := buf.Write([]byte("hello world from test payload"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != 29 {
		t.Fatalf("expected 29 bytes reported written, got %d", n)
	}
	if !buf.Exceeded() {
		t.Fatalf("expected exceeded to be true")
	}
	if buf.String() != "hello worl" {
		t.Fatalf("expected buffer capped at 'hello worl', got '%s'", buf.String())
	}
}
