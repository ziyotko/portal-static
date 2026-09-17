package generator

import (
	"strings"
	"testing"
	"time"

	"portal-static/internal/adapters/caam/model"
)

func TestParseStatBuildsTwelveNormalizedBars(t *testing.T) {
	article := validStatsArticle()
	stat, err := parseStat(article)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Label != "汽车月度销量" || stat.Unit != "万辆" || stat.ReportMonth != 2 {
		t.Fatalf("unexpected stat metadata: %#v", stat)
	}
	if stat.Title != "2026-02 2026年2月汽车工业产销情况简析" {
		t.Fatalf("unexpected stats list title: %q", stat.Title)
	}
	if len(stat.Bars) != 12 || stat.BarsA != "33.3,66.7,0.0,0.0,0.0,0.0,0.0,0.0,0.0,0.0,0.0,0.0" {
		t.Fatalf("unexpected bars: %s %#v", stat.BarsA, stat.Bars)
	}
	if !stat.Bars[1].Active || stat.Bars[0].Active {
		t.Fatal("report month was not highlighted")
	}
}

func TestParseStatRejectsInvalidContract(t *testing.T) {
	tests := []model.Article{
		{ID: "bad-period", Type: 3, Summary: "2026/02", Content: `{}`},
		{ID: "one-series", Type: 3, Summary: "2026-02", Content: `{"label":"x","unit":"辆","series":[{"name":"a","value":1,"months":[1]}]}`},
		{ID: "different-length", Type: 3, Summary: "2026-02", Content: `{"label":"x","unit":"辆","series":[{"name":"a","value":1,"months":[1]},{"name":"b","value":1,"months":[1,2]}]}`},
	}
	for _, article := range tests {
		t.Run(article.ID, func(t *testing.T) {
			if _, err := parseStat(article); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestParseMonthlyStatBuildsTwoYearSeries(t *testing.T) {
	articles := []model.Article{
		{ID: "3", Title: "新能源汽车销量分析", Summary: "2026-07", Content: "200"},
		{ID: "2", Title: "新能源汽车销量分析", Summary: "2026-06", Content: "180"},
		{ID: "1", Title: "新能源汽车销量分析", Summary: "2025-07", Content: "150"},
	}
	stats, err := parseStats(articles, "万辆")
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("unexpected stats count: %d", len(stats))
	}
	stat := stats[0]
	if stat.Title != "2026-07 新能源汽车销量分析" || stat.LegendA != "2025年" || stat.LegendB != "2026年" {
		t.Fatalf("unexpected monthly metadata: %#v", stat)
	}
	if stat.ValueA != "150.0" || stat.ValueB != "380.0" || stat.ReportMonth != 7 {
		t.Fatalf("unexpected monthly values: %#v", stat)
	}
}

func TestArticleViewValidatesStyleLinkAndCover(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	builder := newViewBuilder("https://cdn.example.com/media/", location)
	view := builder.article(model.Article{
		ID:           "1",
		Title:        "标题",
		Summary:      "<p>这是一段摘要</p>",
		Cover:        "images/a.jpg",
		URL:          "javascript:alert(1)",
		DefaultColor: "red;position:absolute",
	}, 0, "fallback.png")
	if view.Clickable || view.Color != "" {
		t.Fatalf("unsafe link or color was accepted: %#v", view)
	}
	if view.Cover != "https://cdn.example.com/media/images/a.jpg" || view.Summary != "这是一段摘要" {
		t.Fatalf("unexpected media or summary: %#v", view)
	}

	valid := builder.article(model.Article{Type: model.ArticleTypeContent, URL: "https://example.com/news", DefaultColor: "#0A5FCE", Cover: "https://img.example/a.jpg"}, 0, "fallback.png")
	if !valid.Clickable || !valid.ExternalLink || string(valid.Color) != "#0A5FCE" || !strings.HasPrefix(valid.Cover, "https://") {
		t.Fatalf("valid values were rejected: %#v", valid)
	}
}

func TestResolveCoverNormalizesCMSUploadPaths(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	builder := newViewBuilder("", location)

	tests := map[string]string{
		"/uploads/article/a.jpg":              "/uploads/article/a.jpg",
		"uploads/article/a.jpg":               "/uploads/article/a.jpg",
		"../../../uploads/article/a.jpg":      "/uploads/article/a.jpg",
		"../../../caam/uploads/article/a.jpg": "/caam/uploads/article/a.jpg",
		"../../caamm/uploads/article/a.jpg":   "/caam/uploads/article/a.jpg",
		"/caamm/uploads/article/a.jpg":        "/caam/uploads/article/a.jpg",
		"caamm/uploads/article/a.jpg":         "/caam/uploads/article/a.jpg",
		"/uploads/2026/05/a.jpg":              "/uploads/2026/05/a.jpg",
		"uploads/2026/05/a.jpg":               "/uploads/2026/05/a.jpg",
		"/caam/uploads/2026/05/a.jpg":         "/caam/uploads/2026/05/a.jpg",
		"https://cdn.example.com/2026/a.jpg":  "https://cdn.example.com/2026/a.jpg",
	}
	for input, want := range tests {
		if got := builder.resolveCover(input, "fallback.png"); got != want {
			t.Fatalf("resolveCover(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveCoverPrefixesConfiguredMediaDomain(t *testing.T) {
	builder := newViewBuilder("https://demo.miic.com.cn", time.UTC)
	want := "https://demo.miic.com.cn/caam/uploads/2026/05/a.jpg"
	if got := builder.resolveCover("/caamm/uploads/2026/05/a.jpg", "fallback.png"); got != want {
		t.Fatalf("resolved cover = %q, want %q", got, want)
	}
}

func validStatsArticle() model.Article {
	return model.Article{
		ID:      "stats-1",
		Type:    3,
		Title:   "2026年2月汽车工业产销情况简析",
		Summary: "2026-02",
		Content: `{"label":"汽车月度销量","unit":"万辆","series":[{"name":"2025年2月","value":30,"months":[10,20]},{"name":"2026年2月","value":45,"months":[15,30]}]}`,
	}
}
