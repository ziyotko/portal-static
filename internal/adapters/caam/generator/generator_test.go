package generator

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
)

type fakeSource struct {
	byKey map[string][]model.Article
	err   error
}

func (f fakeSource) ResolveColumnID(_ context.Context, slot config.SlotConfig) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return 100, nil
}

func (f fakeSource) FetchByColumn(_ context.Context, slot config.SlotConfig) ([]model.Article, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byKey[slot.Key], nil
}

func (f fakeSource) FetchMonthlyStatistics(_ context.Context, _, _ string) ([]model.Article, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f fakeSource) FetchStatisticsTitles(_ context.Context, _ string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	items := f.byKey["stats"]
	titles := make([]string, 0, len(items))
	for _, item := range items {
		if title := strings.TrimSpace(item.Title); title != "" {
			titles = append(titles, title)
		}
	}
	return titles, nil
}

func (f fakeSource) FetchLinksByColumn(_ context.Context, slot config.SlotConfig) ([]model.Article, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byKey[slot.Key], nil
}

func TestGeneratePublishesCompletePageAndRemovesLock(t *testing.T) {
	cfg := testConfig(t)
	if err := os.WriteFile(cfg.Site.Output, []byte("old homepage"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := fakeSource{byKey: map[string][]model.Article{
		cfg.Columns.Stats.Key:    {validStatsArticle()},
		cfg.Columns.Headline.Key: {{ID: "headline", Title: "新头条", URL: "/news/1"}},
	}}
	g, err := New(cfg, source, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	result, err := g.Generate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(cfg.Site.Output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "新头条") || result.TotalItems != 2 {
		t.Fatalf("unexpected generated result: %s %#v", page, result)
	}
	if _, err := os.Stat(cfg.Site.Output + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generation lock was not removed: %v", err)
	}
}

func TestGenerateKeepsOldPageWhenStatsAreInvalid(t *testing.T) {
	cfg := testConfig(t)
	cfg.Site.AllowEmptyStats = false
	old := []byte("old homepage must remain")
	if err := os.WriteFile(cfg.Site.Output, old, 0o644); err != nil {
		t.Fatal(err)
	}
	source := fakeSource{byKey: map[string][]model.Article{
		cfg.Columns.Stats.Key: {{ID: "bad", Summary: "bad", Content: `{}`}},
	}}
	g, _ := New(cfg, source, slog.Default())
	if _, err := g.Generate(context.Background()); err == nil {
		t.Fatal("expected stats validation error")
	}
	got, err := os.ReadFile(cfg.Site.Output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(old) {
		t.Fatalf("old homepage was changed: %q", got)
	}
}

func TestGenerateAllowsEmptyStatsWhenConfigured(t *testing.T) {
	cfg := testConfig(t)
	cfg.Site.AllowEmptyStats = true
	g, err := New(cfg, fakeSource{}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Generate(context.Background()); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(cfg.Site.Output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "暂无统计数据") {
		t.Fatalf("placeholder statistics were not rendered: %s", page)
	}
}

func TestStatisticsMenuEntriesUsesGroupedTitleOrder(t *testing.T) {
	latest := []model.Article{
		{ID: "a", Title: "统计 A", Summary: "2026-06"},
		{ID: "b", Title: "统计 B", Summary: "2026-05"},
	}
	entries := statisticsMenuEntries([]string{"统计 B", "统计 A", "统计 B", " "}, latest)
	if len(entries) != 2 || entries[0].Title != "统计 B" || entries[0].Summary != "2026-05" || entries[1].Title != "统计 A" {
		t.Fatalf("unexpected statistics menu entries: %#v", entries)
	}
}

func TestGenerateReturnsBusyForConcurrentProcess(t *testing.T) {
	cfg := testConfig(t)
	if err := os.WriteFile(cfg.Site.Output+".lock", []byte("busy"), 0o600); err != nil {
		t.Fatal(err)
	}
	g, _ := New(cfg, fakeSource{}, slog.Default())
	_, err := g.Generate(context.Background())
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("expected ErrBusy, got %v", err)
	}
}

func TestProductionTemplateRenders(t *testing.T) {
	templatePath := filepath.Join("..", "testdata", "templates", "home.html.tmpl")
	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	stat, err := parseStat(validStatsArticle())
	if err != nil {
		t.Fatal(err)
	}
	page := PageData{
		GeneratedAt: "2026-07-16T10:00:00+08:00",
		TopNews: []NewsGroupView{
			{Title: "行业要闻", Href: "../first-version/list.html"},
			{Title: "协会活动", Href: "../first-version/list.html"},
			{Title: "通知公告", Href: "../first-version/list.html"},
		},
		Work:        make([]TabView, 6),
		Industry:    make([]TabView, 4),
		Stats:       []StatView{stat},
		InitialStat: stat,
	}
	for i := range page.Work {
		page.Work[i] = TabView{Key: "work-" + string(rune('a'+i)), Title: "协会工作", Href: "../first-version/list.html", Cover: "assets/slice/work-photo.png"}
	}
	for i := range page.Industry {
		page.Industry[i] = TabView{Key: "industry-" + string(rune('a'+i)), Title: "行业新闻", Href: "../first-version/list.html", Cover: "assets/slice/work-photo.png"}
	}
	var output bytes.Buffer
	if err := tpl.Execute(&output, page); err != nil {
		t.Fatal(err)
	}
	if err := validateRenderedPage(output.Bytes()); err != nil {
		t.Fatal(err)
	}
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "home.html.tmpl")
	content := `<!DOCTYPE html><html><body data-generated-at="{{.GeneratedAt}}"><div class="top-news-grid">{{with .Headline}}{{.Title}}{{end}}</div><section class="work-section"></section><section class="industry-section"></section><section class="statistics-section"><button data-stats-news>{{.InitialStat.Title}}</button></section>` + strings.Repeat("x", 1200) + `</body></html>`
	if err := os.WriteFile(templatePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Site.Template = templatePath
	cfg.Site.Output = filepath.Join(dir, "index.html")
	cfg.Site.LockStaleAfter = "1h"
	return cfg
}
