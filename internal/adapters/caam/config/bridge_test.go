package config

import (
	"path/filepath"
	"testing"

	coreconfig "portal-static/internal/core/config"
	"portal-static/internal/core/media"
)

func TestPortalStaticExampleConfigBuildsCompleteCAAMConfig(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "configs", "caam.example.yaml")
	manager, err := coreconfig.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := manager.Bootstrap()
	if snapshot.Driver != "caam" || snapshot.Server.Addr != "127.0.0.1:9142" {
		t.Fatalf("unexpected runtime identity: driver=%q addr=%q", snapshot.Driver, snapshot.Server.Addr)
	}
	cfg, err := FromSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Site.PageName != "首页" || cfg.About.Intro != "协会概况简介" ||
		cfg.WorkPage.Headline != "协会工作头条" || cfg.StatsPage.DomesticChart != "国内数据汽车月度销量" ||
		cfg.MembersPage.Management != "会员专区会员管理" || cfg.PartyPage.Carousel != "党建专区轮播" {
		t.Fatalf("adapter sections were not mapped completely: %#v", cfg)
	}
	if cfg.Site.OutputRoot != snapshot.Paths.SourceRoot || cfg.Site.DistRoot != snapshot.Paths.DistRoot {
		t.Fatalf("unexpected roots: source=%q dist=%q", cfg.Site.OutputRoot, cfg.Site.DistRoot)
	}
	if filepath.Clean(cfg.Site.OutputRoot) == filepath.Clean(cfg.Site.DistRoot) {
		t.Fatal("read-only scaffold and generated output must use different roots")
	}

	resolver, err := media.New(cfg.EffectiveMedia())
	if err != nil {
		t.Fatal(err)
	}
	for input, want := range map[string]string{
		"../../caamm/uploads/a.jpg":                          "/caam/uploads/a.jpg",
		"../../caam/uploads/b.jpg":                           "/caam/uploads/b.jpg",
		"https://demo.miic.com.cn/caamm/uploads/c.jpg":       "/caam/uploads/c.jpg",
		"https://demo.miic.com.cn/caam/uploads/folder/d.jpg": "/caam/uploads/folder/d.jpg",
	} {
		if got := resolver.Resolve(input); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", input, got, want)
		}
	}
}
