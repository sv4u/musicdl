package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashFromBytes_SameContentSameHash(t *testing.T) {
	data := []byte("version: \"1.2\"\ndownload:\n  client_id: x\n  client_secret: y\n")
	h1 := HashFromBytes(data)
	h2 := HashFromBytes(data)
	if h1 != h2 {
		t.Errorf("same content: got hashes %q and %q", h1, h2)
	}
}

func TestHashFromBytes_DifferentContentDifferentHash(t *testing.T) {
	a := []byte("version: \"1.2\"\n")
	b := []byte("version: \"1.0\"\n")
	ha := HashFromBytes(a)
	hb := HashFromBytes(b)
	if ha == hb {
		t.Errorf("different content: got same hash %q", ha)
	}
}

func TestHashFromBytes_EmptyFile(t *testing.T) {
	h := HashFromBytes([]byte{})
	if len(h) != ConfigHashLen {
		t.Errorf("empty input: want %d hex chars, got %d", ConfigHashLen, len(h))
	}
	for _, c := range h {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		t.Errorf("empty input: hash must be hex, got %q", h)
		break
	}
}

func TestHashFromBytes_SmallFile(t *testing.T) {
	h := HashFromBytes([]byte("x"))
	if len(h) != ConfigHashLen {
		t.Errorf("small input: want %d hex chars, got %d", ConfigHashLen, len(h))
	}
}

func TestHashFromBytes_Deterministic(t *testing.T) {
	data := []byte("threads: 4\nspotify:\n  client_id: a\n  client_secret: b\n")
	const iterations = 10
	first := HashFromBytes(data)
	for i := 0; i < iterations; i++ {
		if got := HashFromBytes(data); got != first {
			t.Errorf("iteration %d: got %q, want %q", i, got, first)
		}
	}
}

func TestHashFromPath_Success(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	content := []byte("version: \"1.2\"\ndownload:\n  client_id: id\n  client_secret: sec\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	hashPath, err := HashFromPath(path)
	if err != nil {
		t.Fatalf("HashFromPath: %v", err)
	}
	hashBytes := HashFromBytes(content)
	if hashPath != hashBytes {
		t.Errorf("HashFromPath %q != HashFromBytes %q", hashPath, hashBytes)
	}
	if len(hashPath) != ConfigHashLen {
		t.Errorf("HashFromPath len = %d, want %d", len(hashPath), ConfigHashLen)
	}
}

func TestHashFromPath_MissingFile(t *testing.T) {
	_, err := HashFromPath(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Error("HashFromPath: expected error for missing file")
	}
}
