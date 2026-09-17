package generator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSiteGeneratorDeleteArticleUsesRequestedOutputPath(t *testing.T) {
	cfg := testConfig(t)
	cfg.Site.OutputRoot = t.TempDir()
	cfg.Site.DistRoot = t.TempDir()
	defaultOutput := cfg.Site.DistRoot
	requestedOutput := filepath.Join(t.TempDir(), "custom-output")
	pageCfg := cfg
	pageCfg.Site.OutputRoot = defaultOutput
	pages, err := NewPageGenerator(pageCfg, fakePageSource{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, fakePageSource{}, fakeAboutSource{}, pages, nil, nil)
	target := filepath.Join(requestedOutput, "article", "2026", "08", "101.html")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("article"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := WithOutputPath(context.Background(), requestedOutput+string(filepath.Separator))
	result, err := site.DeleteArticle(ctx, 101)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Deleted || len(result.DeletedPaths) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("requested output article still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(defaultOutput, "article")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("default output was unexpectedly changed: %v", err)
	}
}

func TestSiteGeneratorRejectsOutputPathOverlappingSource(t *testing.T) {
	cfg := testConfig(t)
	cfg.Site.OutputRoot = t.TempDir()
	pages, err := NewPageGenerator(cfg, fakePageSource{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, fakePageSource{}, fakeAboutSource{}, pages, nil, nil)
	if err := site.ValidateOutputPath(filepath.Join(cfg.Site.OutputRoot, "nested")); !errors.Is(err, ErrInvalidOutputPath) {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
