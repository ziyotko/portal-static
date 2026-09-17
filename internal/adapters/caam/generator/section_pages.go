package generator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"portal-static/internal/adapters/caam/model"
)

type WorkTabView struct {
	Key      string
	Title    string
	ListHref string
	Items    []ArticleView
}

type WorkPageData struct {
	GeneratedAt   string
	Focus         []ArticleView
	Association   WorkTabView
	Branch        WorkTabView
	Industry      WorkTabView
	International WorkTabView
	Expo          WorkTabView
	Platforms     []ArticleView
	FooterLinks   []LinkGroupView
}

type StatsPanelView struct {
	Key        string
	Title      string
	ListHref   string
	Active     bool
	Stat       StatView
	Reports    [][]ArticleView
	HasReports bool
}

type StatsPageData struct {
	GeneratedAt string
	Market      []StatsPanelView
	Production  []StatsPanelView
	FooterLinks []LinkGroupView
}

func (g *SiteGenerator) loadWorkColumns(ctx context.Context) ([]PreparedColumn, error) {
	specs := []namedColumnSpec{
		{key: "headline", title: "协会工作头条", name: g.cfg.WorkPage.Headline, types: []int{1, 2}, limit: 2},
		{key: "association", title: "协会动态", name: g.cfg.WorkPage.Association, types: []int{1, 2}, limit: 5},
		{key: "branch", title: "分支机构动态", name: g.cfg.WorkPage.Branch, types: []int{1, 2}, limit: 5},
		{key: "industry", title: "行业发展", name: g.cfg.WorkPage.Industry, types: []int{1, 2}, limit: 5},
		{key: "international", title: "国际合作", name: g.cfg.WorkPage.International, types: []int{1, 2}, limit: 5},
		{key: "expo", title: "展会信息", name: g.cfg.WorkPage.Expo, types: []int{1, 2}, limit: 5},
		{key: "platform", title: "专业平台", name: g.cfg.WorkPage.Platform, types: []int{1, 2}, limit: 5},
	}
	return g.loadNamedColumns(ctx, specs)
}

func (g *SiteGenerator) loadStatsColumns(ctx context.Context) ([]PreparedColumn, error) {
	s := g.cfg.StatsPage
	specs := []namedColumnSpec{
		{key: "domestic-reports", title: "国内数据", name: s.DomesticReports, types: []int{1, 2}, limit: 10},
		{key: "overseas-reports", title: "国外数据", name: s.OverseasReports, types: []int{1, 2}, limit: 10},
		{key: "production-reports", title: "产销", name: s.ProductionReports, types: []int{1, 2}, limit: 10},
		{key: "import-export-reports", title: "进出口", name: s.ImportExportReports, types: []int{1, 2}, limit: 10},
		{key: "domestic-chart", title: "国内数据", name: s.DomesticChart},
		{key: "overseas-chart", title: "国外数据", name: s.OverseasChart},
		{key: "production-chart", title: "产销", name: s.ProductionChart},
		{key: "import-export-chart", title: "进出口", name: s.ImportExportChart},
	}
	return g.loadNamedColumns(ctx, specs)
}

type namedColumnSpec struct {
	key, title, name string
	types            []int
	limit            int
}

func (g *SiteGenerator) loadNamedColumns(ctx context.Context, specs []namedColumnSpec) ([]PreparedColumn, error) {
	columns := make([]PreparedColumn, 0, len(specs))
	for index, spec := range specs {
		var (
			column   model.Column
			articles []model.Article
			err      error
		)
		if spec.limit > 0 {
			column, articles, err = g.aboutSource.FetchPublishedByColumnNameLimit(ctx, spec.name, spec.types, spec.limit)
		} else {
			column, articles, err = g.aboutSource.FetchPublishedByColumnName(ctx, spec.name)
		}
		if errors.Is(err, ErrColumnNotFound) {
			g.logger.Warn("页面栏目不存在，使用空内容", "key", spec.key, "column", spec.name)
			column = model.Column{Name: spec.name}
			articles = nil
		} else if err != nil {
			return nil, err
		}
		if strings.TrimSpace(column.Name) != strings.TrimSpace(spec.name) {
			return nil, fmt.Errorf("column id %d name mismatch: got %q, want %q", column.ID, column.Name, spec.name)
		}
		if len(articles) == 0 {
			g.logger.Warn("页面栏目暂无内容", "key", spec.key, "column", spec.name)
		}
		columns = append(columns, PreparedColumn{Key: spec.key, Title: spec.title, Column: column, Articles: articles})
		reportProgress(ctx, Progress{Stage: "读取页面栏目", Processed: index + 1, Total: len(specs), CurrentColumnID: column.ID})
	}
	return columns, nil
}

func (g *SiteGenerator) pageViews(column PreparedColumn, source, section, fallbackCover string) []ArticleView {
	builder := newViewBuilderForConfig(g.cfg, g.pages.location)
	views := make([]ArticleView, 0, len(column.Articles))
	for index, article := range column.Articles {
		if article.Type == model.ArticleTypeData {
			continue
		}
		view := builder.article(article, index, fallbackCover)
		if href, external, ok := safeHref(strings.TrimSpace(article.URL)); ok && external {
			view.Href, view.Clickable, view.ExternalLink = href, true, true
		} else if href, ok := articleArchiveHref(article, g.pages.location); ok {
			query := url.Values{}
			query.Set("from", source)
			query.Set("section", section)
			view.Href, view.Clickable = href+"?"+query.Encode(), true
		} else {
			g.logger.Warn("页面文章缺少有效详情地址", "column", column.Column.Name, "article_id", article.ID)
			continue
		}
		views = append(views, view)
	}
	return views
}

func columnListHref(column PreparedColumn) string {
	if column.Column.ID <= 0 {
		return ""
	}
	return fmt.Sprintf("list/%d/1.html", column.Column.ID)
}

func (g *SiteGenerator) generateSectionLists(ctx context.Context, byKey map[string]PreparedColumn, keys []string, footerLinks []LinkGroupView) (int, int, error) {
	generated := 0
	totalPages := 0
	seen := make(map[int64]struct{}, len(keys))
	for _, key := range keys {
		prepared := byKey[key]
		columnID := prepared.Column.ID
		if columnID <= 0 {
			continue
		}
		if _, exists := seen[columnID]; exists {
			continue
		}
		seen[columnID] = struct{}{}
		column, articles, err := g.homeSource.FetchColumnArticles(ctx, columnID)
		if err != nil {
			return generated, totalPages, fmt.Errorf("load list column %q: %w", prepared.Column.Name, err)
		}
		result, err := g.pages.generatePreparedList(column, articles, footerLinks)
		if err != nil {
			return generated, totalPages, err
		}
		generated++
		totalPages += result.TotalPages
	}
	return generated, totalPages, nil
}

func (g *SiteGenerator) generateConfiguredSectionLists(ctx context.Context) (int, int, error) {
	specs := []namedColumnSpec{
		{key: "work-association", name: g.cfg.WorkPage.Association, types: []int{1, 2}, limit: 1},
		{key: "work-branch", name: g.cfg.WorkPage.Branch, types: []int{1, 2}, limit: 1},
		{key: "work-industry", name: g.cfg.WorkPage.Industry, types: []int{1, 2}, limit: 1},
		{key: "work-international", name: g.cfg.WorkPage.International, types: []int{1, 2}, limit: 1},
		{key: "work-expo", name: g.cfg.WorkPage.Expo, types: []int{1, 2}, limit: 1},
		{key: "stats-domestic", name: g.cfg.StatsPage.DomesticReports, types: []int{1, 2}, limit: 1},
		{key: "stats-overseas", name: g.cfg.StatsPage.OverseasReports, types: []int{1, 2}, limit: 1},
		{key: "stats-production", name: g.cfg.StatsPage.ProductionReports, types: []int{1, 2}, limit: 1},
		{key: "stats-import-export", name: g.cfg.StatsPage.ImportExportReports, types: []int{1, 2}, limit: 1},
		{key: "members-work", name: g.cfg.MembersPage.Work, types: []int{1, 2}, limit: 1},
		{key: "members-style", name: g.cfg.MembersPage.Style, types: []int{1, 2}, limit: 1},
	}
	columns, err := g.loadNamedColumns(ctx, specs)
	if err != nil {
		return 0, 0, err
	}
	byKey := make(map[string]PreparedColumn, len(columns))
	keys := make([]string, 0, len(columns))
	for _, column := range columns {
		byKey[column.Key] = column
		keys = append(keys, column.Key)
	}
	footerLinks, err := g.pages.fetchFooterLinks(ctx)
	if err != nil {
		return 0, 0, err
	}
	return g.generateSectionLists(ctx, byKey, keys, footerLinks)
}

func (g *SiteGenerator) generateWork(ctx context.Context) (StaticPageResult, error) {
	started := time.Now()
	if err := g.prepareDist(); err != nil {
		return StaticPageResult{}, err
	}
	columns, err := g.loadWorkColumns(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	footerLinks, err := g.pages.fetchFooterLinks(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	byKey := make(map[string]PreparedColumn, len(columns))
	total := 0
	for _, column := range columns {
		byKey[column.Key] = column
		total += len(column.Articles)
	}
	fallback := "assets/images/content-placeholder.png"
	focus := g.pageViews(byKey["headline"], "work", "headline", fallback)
	if len(focus) > 2 {
		focus = focus[:2]
	}
	association := g.pageViews(byKey["association"], "work", "association", fallback)
	data := WorkPageData{
		GeneratedAt: time.Now().In(g.pages.location).Format(time.RFC3339), Focus: focus,
		Association:   WorkTabView{Key: "association", Title: "协会动态", ListHref: columnListHref(byKey["association"]), Items: association},
		Branch:        WorkTabView{Key: "branch", Title: "分支机构动态", ListHref: columnListHref(byKey["branch"]), Items: g.pageViews(byKey["branch"], "work", "branch", fallback)},
		Industry:      WorkTabView{Key: "industry", Title: "行业发展", ListHref: columnListHref(byKey["industry"]), Items: g.pageViews(byKey["industry"], "work", "industry", fallback)},
		International: WorkTabView{Key: "international", Title: "国际合作", ListHref: columnListHref(byKey["international"]), Items: g.pageViews(byKey["international"], "work", "international", fallback)},
		Expo:          WorkTabView{Key: "expo", Title: "展会信息", ListHref: columnListHref(byKey["expo"]), Items: g.pageViews(byKey["expo"], "work", "expo", fallback)},
		Platforms:     g.pageViews(byKey["platform"], "work", "platform", fallback),
		FooterLinks:   footerLinks,
	}
	output, err := g.renderSectionPage(g.cfg.WorkPage.Template, g.cfg.WorkPage.Output, data, "class=\"work-main\"")
	if err != nil {
		return StaticPageResult{}, err
	}
	return StaticPageResult{
		GeneratedAt: time.Now().In(g.pages.location), DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000,
		Page: "work", TotalItems: total, Output: output, Gray: grayCode(g.grayscale),
	}, nil
}

func (g *SiteGenerator) generateStats(ctx context.Context) (StaticPageResult, error) {
	started := time.Now()
	if err := g.prepareDist(); err != nil {
		return StaticPageResult{}, err
	}
	columns, err := g.loadStatsColumns(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	footerLinks, err := g.pages.fetchFooterLinks(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	byKey := make(map[string]PreparedColumn, len(columns))
	total := 0
	for _, column := range columns {
		total += len(column.Articles)
		byKey[column.Key] = column
	}
	panels := make(map[string]StatsPanelView, 4)
	for _, spec := range []struct{ key, title string }{
		{"domestic", "国内数据"}, {"overseas", "国外数据"},
		{"production", "产销"}, {"import-export", "进出口"},
	} {
		chartColumn := byKey[spec.key+"-chart"]
		dataArticles := make([]model.Article, 0, len(chartColumn.Articles))
		for _, article := range chartColumn.Articles {
			if article.Type == model.ArticleTypeData {
				dataArticles = append(dataArticles, article)
			}
		}
		stat := emptyStatView()
		if parsed, parseErr := parseStats(dataArticles, g.cfg.StatsPage.Unit); parseErr == nil && len(parsed) > 0 {
			stat = parsed[0]
		} else if parseErr != nil {
			g.logger.Warn("统计页面板无有效图表数据", "column", chartColumn.Column.Name, "error", parseErr)
		}
		reportColumn := byKey[spec.key+"-reports"]
		reportViews := g.pageViews(reportColumn, "stats", spec.key, "")
		panels[spec.key] = StatsPanelView{
			Key: spec.key, Title: spec.title, ListHref: columnListHref(reportColumn), Stat: stat,
			Reports: splitAboutColumns(reportViews), HasReports: len(reportViews) > 0,
		}
	}
	domestic := panels["domestic"]
	domestic.Active = true
	production := panels["production"]
	production.Active = true
	data := StatsPageData{
		GeneratedAt: time.Now().In(g.pages.location).Format(time.RFC3339),
		Market:      []StatsPanelView{domestic, panels["overseas"]},
		Production:  []StatsPanelView{production, panels["import-export"]},
		FooterLinks: footerLinks,
	}
	output, err := g.renderSectionPage(g.cfg.StatsPage.Template, g.cfg.StatsPage.Output, data, "class=\"statistics-main\"")
	if err != nil {
		return StaticPageResult{}, err
	}
	return StaticPageResult{
		GeneratedAt: time.Now().In(g.pages.location), DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000,
		Page: "stats", TotalItems: total, Output: output, Gray: grayCode(g.grayscale),
	}, nil
}

func (g *SiteGenerator) renderSectionPage(templatePath, outputName string, data any, marker string) (string, error) {
	output := filepath.Join(g.cfg.Site.DistRoot, outputName)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return "", fmt.Errorf("create section page output directory: %w", err)
	}
	lock, err := acquireFileLock(output+".lock", g.pages.staleAfter)
	if err != nil {
		return "", err
	}
	defer lock.release()
	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("parse section page template: %w", err)
	}
	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("render section page: %w", err)
	}
	renderedPage, err := applyGrayscale(rendered.Bytes(), g.grayscale)
	if err != nil {
		return "", err
	}
	rendered.Reset()
	_, _ = rendered.Write(renderedPage)
	if err := validateInnerPage(rendered.Bytes(), marker); err != nil {
		return "", fmt.Errorf("validate section page: %w", err)
	}
	if bytes.Contains(rendered.Bytes(), []byte("href=\"#\"")) {
		return "", errors.New("rendered section page contains placeholder links")
	}
	if err := publishFile(output, rendered.Bytes()); err != nil {
		return "", err
	}
	return output, nil
}
