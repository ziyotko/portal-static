package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExampleConfigLoadsWithProductionColumnNames(t *testing.T) {
	path := filepath.Join("..", "testdata", "config.example.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Columns.TopNews[0].Name != "首页行业要闻" || cfg.Columns.Stats.Name != "首页统计数据" {
		t.Fatalf("unexpected production column mapping: %#v", cfg.Columns)
	}
	if cfg.Site.PageName != "首页" || !cfg.Site.AllowEmptyStats {
		t.Fatalf("unexpected homepage generation settings: %#v", cfg.Site)
	}
	if len(cfg.Columns.FooterLinks) != 3 || cfg.Columns.FooterLinks[0].Name != "首页合作协会" {
		t.Fatalf("unexpected footer link mapping: %#v", cfg.Columns.FooterLinks)
	}
	if !filepath.IsAbs(cfg.Site.Template) || !filepath.IsAbs(cfg.Site.Output) ||
		!filepath.IsAbs(cfg.Site.ListTemplate) || !filepath.IsAbs(cfg.Site.ArticleTemplate) ||
		!filepath.IsAbs(cfg.Site.OutputRoot) || !filepath.IsAbs(cfg.Site.DistRoot) ||
		!filepath.IsAbs(cfg.About.Template) || !filepath.IsAbs(cfg.WorkPage.Template) ||
		!filepath.IsAbs(cfg.StatsPage.Template) || !filepath.IsAbs(cfg.MembersPage.Template) ||
		!filepath.IsAbs(cfg.PartyPage.Template) {
		t.Fatal("all template and output paths must resolve relative to the config file")
	}
	if cfg.About.Intro != "协会概况简介" || cfg.About.RegularMember != "协会概况普通会员" || cfg.About.Output != "about.html" {
		t.Fatalf("unexpected about page mapping: %#v", cfg.About)
	}
	if cfg.WorkPage.Headline != "协会工作头条" || cfg.WorkPage.Platform != "协会工作专业平台" || cfg.WorkPage.Output != "work.html" {
		t.Fatalf("unexpected work page mapping: %#v", cfg.WorkPage)
	}
	if cfg.StatsPage.DomesticReports != "统计数据国内数据" || cfg.StatsPage.DomesticChart != "国内数据汽车月度销量" || cfg.StatsPage.Output != "stats.html" {
		t.Fatalf("unexpected stats page mapping: %#v", cfg.StatsPage)
	}
	if cfg.MembersPage.Work != "会员专区会员工作" || cfg.MembersPage.Management != "会员专区会员管理" || cfg.MembersPage.Member != "会员专区会员单位" || cfg.MembersPage.Output != "members.html" {
		t.Fatalf("unexpected members page mapping: %#v", cfg.MembersPage)
	}
	if cfg.PartyPage.Work != "党建专区工作动态" || cfg.PartyPage.Carousel != "党建专区轮播" || cfg.PartyPage.Output != "party.html" {
		t.Fatalf("unexpected party page mapping: %#v", cfg.PartyPage)
	}
	if cfg.Site.PageSize != 10 {
		t.Fatalf("unexpected page size: %d", cfg.Site.PageSize)
	}
	if cfg.Server.BatchIdleTimeout != "15m" || cfg.Server.BatchMaxDuration != "6h" {
		t.Fatalf("unexpected batch timeout settings: %#v", cfg.Server)
	}
	if cfg.Columns.Carousel.FallbackCover != "assets/images/content-placeholder.png" ||
		cfg.Columns.Videos.FallbackCover != "assets/images/content-placeholder.png" {
		t.Fatalf("unexpected content placeholder mapping: carousel=%q videos=%q", cfg.Columns.Carousel.FallbackCover, cfg.Columns.Videos.FallbackCover)
	}
}

func TestBatchTimeoutDefaultsAndValidation(t *testing.T) {
	cfg := Default()
	if cfg.Site.PageName != "首页" || !cfg.Site.AllowEmptyStats {
		t.Fatalf("unexpected homepage defaults: %#v", cfg.Site)
	}
	if cfg.Server.BatchIdleTimeout != "15m" || cfg.Server.BatchMaxDuration != "6h" {
		t.Fatalf("unexpected batch timeout defaults: %#v", cfg.Server)
	}
	cfg.Server.BatchMaxDuration = "10m"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "batch_max_duration") {
		t.Fatalf("expected batch duration ordering error, got %v", err)
	}
}

func TestPageNameIsRequired(t *testing.T) {
	cfg := Default()
	cfg.Site.PageName = " "
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "page_name") {
		t.Fatalf("expected page_name validation error, got %v", err)
	}
}
