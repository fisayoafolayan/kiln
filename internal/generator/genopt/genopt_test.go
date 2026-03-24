package genopt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeChecksum(t *testing.T) {
	content := []byte("hello world")
	sum := computeChecksum(content)
	if len(sum) != 64 {
		t.Fatalf("expected 64-char hex digest, got %d chars: %s", len(sum), sum)
	}
	// Same input must produce same output.
	if computeChecksum(content) != sum {
		t.Fatal("checksum is not deterministic")
	}
	// Different input must produce different output.
	if computeChecksum([]byte("hello world!")) == sum {
		t.Fatal("different content produced same checksum")
	}
}

func TestEmbedChecksum(t *testing.T) {
	content := []byte("// kiln:checksum=__CHECKSUM__\npackage foo\n")
	result := embedChecksum(content)

	// Placeholder must be replaced.
	if string(result) == string(content) {
		t.Fatal("placeholder was not replaced")
	}

	// Must contain a 64-char hex digest.
	m := checksumRe.FindSubmatch(result)
	if m == nil {
		t.Fatal("result does not contain a valid checksum")
	}
	if len(m[1]) != 64 {
		t.Fatalf("expected 64-char digest, got %d", len(m[1]))
	}
}

func TestFileIsUserModified_NonExistent(t *testing.T) {
	modified, err := FileIsUserModified("/nonexistent/path/file.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified {
		t.Fatal("nonexistent file should not be reported as modified")
	}
}

func TestFileIsUserModified_NoChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	os.WriteFile(path, []byte("package foo\n"), 0644)

	modified, err := FileIsUserModified(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified {
		t.Fatal("file without checksum should not be reported as modified")
	}
}

func TestFileIsUserModified_Unmodified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	// Write a file with a valid embedded checksum.
	content := []byte("// kiln:checksum=__CHECKSUM__\npackage foo\n")
	final := embedChecksum(content)
	os.WriteFile(path, final, 0644)

	modified, err := FileIsUserModified(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified {
		t.Fatal("unmodified file should not be reported as modified")
	}
}

func TestFileIsUserModified_Modified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	// Write a file with a valid embedded checksum, then modify it.
	content := []byte("// kiln:checksum=__CHECKSUM__\npackage foo\n")
	final := embedChecksum(content)
	// Append some user edit.
	final = append(final, []byte("// user edit\n")...)
	os.WriteFile(path, final, 0644)

	modified, err := FileIsUserModified(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !modified {
		t.Fatal("modified file should be reported as modified")
	}
}

func TestFileIsUserModified_LiteralPlaceholder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	// File with the literal __CHECKSUM__ (never had a real checksum).
	os.WriteFile(path, []byte("// kiln:checksum=__CHECKSUM__\npackage foo\n"), 0644)

	modified, err := FileIsUserModified(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified {
		t.Fatal("file with literal placeholder should not be reported as modified (no valid hex digest)")
	}
}
