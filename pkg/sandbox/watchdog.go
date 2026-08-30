package sandbox

import (
	"bytes"
	"io"
	"sync"
)

// BoundedBuffer captures output stream data up to a fixed maximum byte limit to prevent memory bloat.
type BoundedBuffer struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	maxBytes int64
	written  int64
	exceeded bool
}

// NewBoundedBuffer creates a new BoundedBuffer with the specified byte cap.
func NewBoundedBuffer(maxBytes int64) *BoundedBuffer {
	if maxBytes <= 0 {
		maxBytes = 1048576 // 1MB default
	}
	return &BoundedBuffer{
		maxBytes: maxBytes,
	}
}

// Write appends bytes up to the configured limit.
func (b *BoundedBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n = len(p)
	b.written += int64(n)

	if int64(b.buf.Len()) < b.maxBytes {
		remaining := b.maxBytes - int64(b.buf.Len())
		if int64(len(p)) <= remaining {
			b.buf.Write(p)
		} else {
			b.buf.Write(p[:remaining])
			b.exceeded = true
		}
	} else {
		b.exceeded = true
	}

	return n, nil
}

// String returns the captured output as a string.
func (b *BoundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Exceeded returns whether the output exceeded the byte limit.
func (b *BoundedBuffer) Exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exceeded
}

// TotalWritten returns the total number of bytes passed to Write.
func (b *BoundedBuffer) TotalWritten() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.written
}

var _ io.Writer = (*BoundedBuffer)(nil)
