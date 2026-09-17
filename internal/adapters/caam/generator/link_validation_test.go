package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSiteInternalLinks(t *testing.T) {
	root := t.TempDir()
	mustWriteTestFile(t, filepath.Join(root, "index.html"), `<a href="list/14/1.html">list</a><a href="article/2026/08/1.html?from=home">article</a>`)
	mustWriteTestFile(t, filepath.Join(root, "list", "14", "1.html"), `<a href="../../article/2026/08/1.html?from=list">article</a>`)
	mustWriteTestFile(t, filepath.Join(root, "article", "2026", "08", "1.html"), `<main>article</main>`)

	validated, err := validateSiteInternalLinks(root, []string{"index.html"})
	if err != nil {
		t.Fatalf("validate generated links: %v", err)
	}
	if validated != 2 {
		t.Fatalf("validated targets = %d, want 2", validated)
	}
}

func TestValidateSiteInternalLinksRejectsMissingTarget(t *testing.T) {
	root := t.TempDir()
	mustWriteTestFile(t, filepath.Join(root, "index.html"), `<a href="article/2026/08/missing.html">missing</a>`)
	if err := os.MkdirAll(filepath.Join(root, "list"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := validateSiteInternalLinks(root, []string{"index.html"}); err == nil || !strings.Contains(err.Error(), "missing target") {
		t.Fatalf("error = %v, want missing target", err)
	}
}

func mustWriteTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
