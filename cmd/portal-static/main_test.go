package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"portal-static/internal/core/config"
	"portal-static/internal/core/httpapi"
	"portal-static/internal/platform"
)

type commandFactory struct {
	mode   platform.Mode
	closed bool
}

func (f *commandFactory) Driver() string { return "test" }
func (f *commandFactory) Build(_ context.Context, mode platform.Mode, _ config.Snapshot, _ *slog.Logger) (*platform.Runtime, error) {
	f.mode = mode
	operation := func(context.Context) (any, error) { return map[string]any{"generated_files": 1}, nil }
	article := func(context.Context, int64) (any, error) { return nil, nil }
	operations := httpapi.Operations{
		GenerateSite: operation, GeneratePages: operation, GenerateAllLists: operation, GenerateAllArticles: operation,
		GeneratePage: func(context.Context, string) (any, error) { return nil, nil },
		GenerateList: article, GenerateListByName: func(context.Context, string) (any, error) { return nil, nil },
		GenerateArticle: article, DeleteArticle: article, GenerateArticleRelated: article, DeleteArticleRelated: article,
		NormalizePageName: func(name string) (string, bool) { return name, true }, PageNameError: "invalid page",
		ValidateOutputPath: func(string) error { return nil }, ClassifyError: func(error) (int, string, bool) { return 0, "", false },
	}
	return platform.NewRuntime(operations, func() error { f.closed = true; return nil })
}

func TestGenerateCommandFlowAlwaysUsesWholeSiteOperation(t *testing.T) {
	siteCalls, pageCalls := 0, 0
	operations := httpapi.Operations{
		GenerateSite: func(context.Context) (any, error) {
			siteCalls++
			return map[string]any{"generated_files": 1}, nil
		},
		GeneratePage: func(context.Context, string) (any, error) {
			pageCalls++
			return nil, nil
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, driver := range []string{"miic", "caam"} {
		if err := generateSite("preview", config.Snapshot{Document: config.Document{Driver: driver, Site: config.SiteConfig{ID: driver}}}, operations, logger); err != nil {
			t.Fatal(err)
		}
	}
	if siteCalls != 2 || pageCalls != 0 {
		t.Fatalf("site calls=%d page calls=%d", siteCalls, pageCalls)
	}
}

func TestPreviewUsesRegisteredFactoryWithoutDatabaseConfiguration(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "site.yaml")
	content := `version: 1
driver: test
site:
  id: test
  timezone: Asia/Shanghai
server:
  addr: 127.0.0.1:9999
  token_env: TEST_TOKEN
  request_timeout: 1m
  batch_idle_timeout: 1m
  batch_max_duration: 1h
paths:
  source_root: source
  dist_root: dist/site
  preview_root: dist/preview
  templates:
    home: home.tmpl
media:
  mode: same_origin
adapter: {}
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	factory := &commandFactory{}
	registry := platform.NewRegistry()
	if err := registry.Register(factory); err != nil {
		t.Fatal(err)
	}
	if err := runWithRegistry([]string{"portal-static", "preview", "--config", configPath}, registry); err != nil {
		t.Fatal(err)
	}
	if factory.mode != platform.Preview || !factory.closed {
		t.Fatalf("mode=%q closed=%v", factory.mode, factory.closed)
	}
}
