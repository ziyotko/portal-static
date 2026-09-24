package generator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"portal-static/internal/contracts"
)

func TestSiteGeneratorDeleteArticleUsesRequestedOutputPath(t *testing.T) {
	cfg := testConfig(t)
	cfg.Site.OutputRoot = t.TempDir()
	cfg.Site.DistRoot = t.TempDir()
	defaultOutput := cfg.Site.DistRoot
	requestedOutput := filepath.Join(t.TempDir(), "custom-output")
	cfg.Site.DistRoot = requestedOutput
	pageCfg := cfg
	pageCfg.Site.OutputRoot = requestedOutput
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

func TestGenerateSiteUsesRequestedOutputPathThroughStaging(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	cfg.Site.AllowEmptyStats = true
	requestedOutput := filepath.Join(t.TempDir(), "custom-output")
	cfg.Site.DistRoot = requestedOutput
	pageCfg := cfg
	pageCfg.Site.OutputRoot = cfg.Site.DistRoot
	pages, err := NewPageGenerator(pageCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	home, err := New(cfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)
	ctx := contracts.WithOptions(context.Background(), requestedOutput, false)

	result, err := site.GenerateSite(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(result.Output) != filepath.Clean(requestedOutput) {
		t.Fatalf("output = %q, want %q", result.Output, requestedOutput)
	}
	for _, name := range []string{"index.html", "about.html", "work.html", "stats.html", "members.html", "party.html"} {
		if _, err := os.Stat(filepath.Join(requestedOutput, name)); err != nil {
			t.Fatalf("requested output is missing %s: %v", name, err)
		}
	}
}
