package generator

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyGrayscaleAddsWholePageFilterAndIsIdempotent(t *testing.T) {
	input := []byte(`<!DOCTYPE html><html lang="zh-CN"><head><title>test</title></head><body>page</body></html>`)
	page, err := applyGrayscale(input, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range [][]byte{[]byte(grayscaleMarker), []byte(`id="site-grayscale"`), []byte(`filter:grayscale(100%)`)} {
		if !bytes.Contains(page, marker) {
			t.Fatalf("grayscale page is missing %q: %s", marker, page)
		}
	}
	again, err := applyGrayscale(page, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(page, again) {
		t.Fatal("grayscale application must be idempotent")
	}
}

func TestGenerateHomeSupportsGrayscale(t *testing.T) {
	cfg, aboutSource, homeSource := siteTestFixture(t)
	cfg.Site.AllowEmptyStats = true
	distCfg := cfg
	distCfg.Site.OutputRoot = cfg.Site.DistRoot
	distCfg.Site.Output = filepath.Join(cfg.Site.DistRoot, "index.html")
	pages, err := NewPageGenerator(distCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	home, err := New(distCfg, homeSource, nil)
	if err != nil {
		t.Fatal(err)
	}
	site := NewSiteGenerator(cfg, homeSource, aboutSource, pages, home, nil)
	result, err := site.GeneratePage(WithGrayscale(context.Background(), true), "home")
	if err != nil {
		t.Fatal(err)
	}
	if result.Gray != "1" {
		t.Fatalf("unexpected result: %#v", result)
	}
	page, err := os.ReadFile(filepath.Join(cfg.Site.DistRoot, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(page, []byte(grayscaleMarker)) || !bytes.Contains(page, []byte(`id="site-grayscale"`)) {
		t.Fatal("generated homepage is not grayscale")
	}
}
