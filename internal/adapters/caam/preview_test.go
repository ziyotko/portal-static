package caam

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	caamconfig "portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/demo"
	coreconfig "portal-static/internal/core/config"
)

func TestPreviewSourceGeneratesCompleteStandaloneSite(t *testing.T) {
	manager, err := coreconfig.Open(filepath.Join("..", "..", "..", "configs", "caam.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := caamconfig.FromSnapshot(manager.Bootstrap())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Site.DistRoot = filepath.Join(t.TempDir(), "caam-preview")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	site, err := newSiteGenerator(cfg, demo.NewSiteSource(cfg), logger)
	if err != nil {
		t.Fatal(err)
	}
	result, err := site.GenerateSite(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.GeneratedFiles <= 6 || result.GeneratedDetails == 0 || result.GeneratedLists == 0 {
		t.Fatalf("preview is not a complete site: %#v", result)
	}
	for _, path := range []string{
		"index.html", "about.html", "work.html", "stats.html", "members.html", "party.html",
		filepath.Join("css", "style.css"), filepath.Join("js", "index.js"),
	} {
		info, statErr := os.Stat(filepath.Join(cfg.Site.DistRoot, path))
		if statErr != nil || info.IsDir() || info.Size() == 0 {
			t.Errorf("missing standalone preview file %s: %v", path, statErr)
		}
	}
	for _, directory := range []string{"article", "list"} {
		entries, readErr := os.ReadDir(filepath.Join(cfg.Site.DistRoot, directory))
		if readErr != nil || len(entries) == 0 {
			t.Errorf("preview directory %s is empty: %v", directory, readErr)
		}
	}
}
