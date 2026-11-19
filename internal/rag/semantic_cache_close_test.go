package rag

import "testing"

type closingBackend struct {
	*MemoryCacheBackend
	closed int
}

func (b *closingBackend) Close() error { b.closed++; return nil }

func TestSPEC_CORE_73_R7_CloseReleasesBackend(t *testing.T) {
	b := &closingBackend{MemoryCacheBackend: NewMemoryCacheBackend()}
	c := newTestCache(t, b)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if b.closed != 1 {
		t.Fatalf("backend closed %d times, want 1", b.closed)
	}
	// A backend without connections is a no-op.
	if err := newTestCache(t, NewMemoryCacheBackend()).Close(); err != nil {
		t.Fatalf("memory backend Close: %v", err)
	}
}
