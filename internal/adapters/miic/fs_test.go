package miic

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPublishFileUsesWebReadablePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), "article", "101.html")
	if err := publishFile(path, []byte("page")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("file permissions = %04o, want 0644", got)
	}
}

func TestReplaceDirectoryUsesWebReadablePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	parent := t.TempDir()
	staging, err := os.MkdirTemp(parent, ".staging-*")
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(staging, "article", "2026", "09")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "101.html"), []byte("page"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "site")
	if err := replaceDirectory(staging, target); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{target, filepath.Join(target, "article"), filepath.Join(target, "article", "2026"), filepath.Join(target, "article", "2026", "09")} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o755 {
			t.Fatalf("directory %s permissions = %04o, want 0755", directory, got)
		}
	}
	info, err := os.Stat(filepath.Join(target, "article", "2026", "09", "101.html"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("file permissions = %04o, want 0644", got)
	}
}
