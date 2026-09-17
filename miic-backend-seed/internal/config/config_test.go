package config

import (
	"path/filepath"
	"testing"
)

func TestExampleConfigLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Addr != "127.0.0.1:9143" || cfg.Site.PageName != "资讯动态" || !cfg.News.DynamicColumns || cfg.Business.PageName != "核心业务" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestValidateRejectsMissingStaticCategories(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.News.DynamicColumns = false
	cfg.News.Categories = nil
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing category error")
	}
}

func TestValidateRejectsOutputOutsideDistInsideSource(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Site.DistRoot = filepath.Join(cfg.Site.SourceRoot, "published")
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected unsafe output error")
	}
}
