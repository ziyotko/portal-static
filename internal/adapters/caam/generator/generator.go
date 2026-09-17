package generator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
	"portal-static/internal/adapters/caam/repository"
)

type Result struct {
	GeneratedAt     time.Time      `json:"generated_at"`
	DurationSeconds float64        `json:"duration_seconds"`
	Output          string         `json:"output"`
	TotalItems      int            `json:"total_items"`
	Columns         map[string]int `json:"columns"`
}

type Generator struct {
	cfg        config.Config
	source     repository.ArticleSource
	logger     *slog.Logger
	location   *time.Location
	staleAfter time.Duration
	grayscale  bool
}

func New(cfg config.Config, source repository.ArticleSource, logger *slog.Logger) (*Generator, error) {
	location, err := time.LoadLocation(cfg.Site.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load site timezone: %w", err)
	}
	staleAfter, err := time.ParseDuration(cfg.Site.LockStaleAfter)
	if err != nil {
		return nil, fmt.Errorf("parse lock stale duration: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Generator{cfg: cfg, source: source, logger: logger, location: location, staleAfter: staleAfter}, nil
}

func (g *Generator) Generate(ctx context.Context) (Result, error) {
	started := time.Now()
	if err := os.MkdirAll(filepath.Dir(g.cfg.Site.Output), 0o755); err != nil {
		return Result{}, fmt.Errorf("create output directory: %w", err)
	}
	lock, err := acquireFileLock(g.cfg.Site.Output+".lock", g.staleAfter)
	if err != nil {
		return Result{}, err
	}
	defer lock.release()

	data := make(map[string][]model.Article)
	listColumnIDs := make(map[string]int64)
	counts := make(map[string]int)
	total := 0
	contentSlots := g.cfg.ContentSlots()
	for index, slot := range contentSlots {
		articles, err := g.source.FetchByColumn(ctx, slot)
		missingColumn := errors.Is(err, repository.ErrColumnNotFound)
		if missingColumn {
			g.logger.Warn("首页栏目不存在，使用空内容", "key", slot.Key, "column", slot.Name)
			articles = nil
		} else if err != nil {
			return Result{}, err
		}
		if slot.Key == g.cfg.Columns.Stats.Key && !missingColumn {
			titles, titleErr := g.source.FetchStatisticsTitles(ctx, slot.Name)
			if titleErr != nil {
				return Result{}, titleErr
			}
			articles = statisticsMenuEntries(titles, articles)
		}
		var columnID int64
		if len(articles) > 0 {
			columnID = articles[0].ColumnID
		}
		if columnID <= 0 && !missingColumn {
			columnID, err = g.source.ResolveColumnID(ctx, slot)
			if errors.Is(err, repository.ErrColumnNotFound) {
				g.logger.Warn("首页栏目不存在，使用空内容", "key", slot.Key, "column", slot.Name)
				columnID = 0
			} else if err != nil {
				return Result{}, err
			}
		}
		listColumnIDs[slot.Key] = columnID
		data[slot.Key] = articles
		counts[slot.Key] = len(articles)
		total += len(articles)
		reportProgress(ctx, Progress{Stage: "读取首页栏目", Processed: index + 1, Total: len(contentSlots)})
	}
	for index, slot := range g.cfg.Columns.FooterLinks {
		links, err := g.source.FetchLinksByColumn(ctx, slot)
		if errors.Is(err, repository.ErrColumnNotFound) {
			g.logger.Warn("首页友链栏目不存在，使用空内容", "key", slot.Key, "column", slot.Name)
			links = nil
		} else if err != nil {
			return Result{}, err
		}
		data[slot.Key] = links
		counts[slot.Key] = len(links)
		total += len(links)
		reportProgress(ctx, Progress{Stage: "读取首页友链", Processed: index + 1, Total: len(g.cfg.Columns.FooterLinks)})
	}

	stats, err := g.buildStatistics(ctx, data[g.cfg.Columns.Stats.Key])
	if err != nil {
		if !g.cfg.Site.AllowEmptyStats {
			return Result{}, err
		}
		g.logger.Warn("统计数据为空或无效，使用占位数据", "error", err)
		stats = []StatView{emptyStatView()}
	}
	builder := newViewBuilderForConfig(g.cfg, g.location)
	page := PageData{
		GeneratedAt: time.Now().In(g.location).Format(time.RFC3339),
		Carousel:    builder.articles(data[g.cfg.Columns.Carousel.Key], g.cfg.Columns.Carousel.FallbackCover),
		TopNews:     buildGroups(g.cfg.Columns.TopNews, data, builder, listColumnIDs),
		Work:        buildTabs(g.cfg.Columns.Work, data, builder, listColumnIDs),
		Industry:    buildTabs(g.cfg.Columns.Industry, data, builder, listColumnIDs),
		Stats:       stats,
		InitialStat: stats[0],
		Topics:      builder.articles(data[g.cfg.Columns.Topics.Key], g.cfg.Columns.Topics.FallbackCover),
		Videos:      builder.articles(data[g.cfg.Columns.Videos.Key], g.cfg.Columns.Videos.FallbackCover),
		FooterLinks: buildLinkGroups(g.cfg.Columns.FooterLinks, data, builder),
	}
	headline := builder.articles(data[g.cfg.Columns.Headline.Key], g.cfg.Columns.Headline.FallbackCover)
	if len(headline) > 0 {
		page.Headline = &headline[0]
	}

	if err := g.renderAndPublish(page, g.grayscale || Grayscale(ctx)); err != nil {
		return Result{}, err
	}
	result := Result{
		GeneratedAt:     time.Now().In(g.location),
		DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000,
		Output:          g.cfg.Site.Output,
		TotalItems:      total,
		Columns:         counts,
	}
	g.logger.Info("首页生成成功", "output", result.Output, "items", result.TotalItems, "duration_seconds", result.DurationSeconds)
	return result, nil
}

func statisticsMenuEntries(titles []string, latest []model.Article) []model.Article {
	byTitle := make(map[string]model.Article, len(latest))
	for _, article := range latest {
		title := strings.TrimSpace(article.Title)
		if title == "" {
			continue
		}
		if _, exists := byTitle[title]; !exists {
			byTitle[title] = article
		}
	}
	entries := make([]model.Article, 0, len(titles))
	seen := make(map[string]struct{}, len(titles))
	for _, value := range titles {
		title := strings.TrimSpace(value)
		if title == "" {
			continue
		}
		if _, exists := seen[title]; exists {
			continue
		}
		seen[title] = struct{}{}
		entry := byTitle[title]
		entry.Title = title
		entries = append(entries, entry)
	}
	return entries
}

func (g *Generator) buildStatistics(ctx context.Context, entries []model.Article) ([]StatView, error) {
	stats := make([]StatView, 0, len(entries))
	var buildErrors []error
	for _, entry := range entries {
		history, err := g.source.FetchMonthlyStatistics(ctx, entry.Title, entry.Summary)
		if err == nil {
			if monthly, monthlyErr := parseMonthlyStat(history, g.cfg.Columns.Stats.Unit); monthlyErr == nil {
				stats = append(stats, monthly)
				continue
			} else {
				err = monthlyErr
			}
		}

		// Keep compatibility with the original single-record JSON contract.
		if legacy, legacyErr := parseStat(entry); legacyErr == nil {
			stats = append(stats, legacy)
			continue
		} else if err == nil {
			err = legacyErr
		}
		buildErrors = append(buildErrors, fmt.Errorf("build statistics entry %s: %w", entry.ID, err))
	}
	if len(stats) == 0 {
		if len(buildErrors) > 0 {
			return nil, fmt.Errorf("statistics column has no valid records: %w", errors.Join(buildErrors...))
		}
		return nil, errors.New("statistics column has no records")
	}
	return stats, nil
}

func (g *Generator) renderAndPublish(page PageData, grayscale bool) error {
	tpl, err := template.ParseFiles(g.cfg.Site.Template)
	if err != nil {
		return fmt.Errorf("parse homepage template: %w", err)
	}
	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, page); err != nil {
		return fmt.Errorf("render homepage template: %w", err)
	}
	renderedPage, err := applyGrayscale(rendered.Bytes(), grayscale)
	if err != nil {
		return err
	}
	rendered.Reset()
	_, _ = rendered.Write(renderedPage)
	if err := validateRenderedPage(rendered.Bytes()); err != nil {
		return err
	}

	dir := filepath.Dir(g.cfg.Site.Output)
	tempFile, err := os.CreateTemp(dir, ".caam-index-*.tmp")
	if err != nil {
		return fmt.Errorf("create homepage temporary file: %w", err)
	}
	tempPath := tempFile.Name()
	cleanupTemp := true
	defer func() {
		_ = tempFile.Close()
		if cleanupTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := tempFile.Write(rendered.Bytes()); err != nil {
		return fmt.Errorf("write homepage temporary file: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("sync homepage temporary file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close homepage temporary file: %w", err)
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return fmt.Errorf("set homepage permissions: %w", err)
	}

	backupPath := g.cfg.Site.Output + ".bak"
	_ = os.Remove(backupPath)
	hadOutput := false
	if _, err := os.Stat(g.cfg.Site.Output); err == nil {
		hadOutput = true
		if err := os.Rename(g.cfg.Site.Output, backupPath); err != nil {
			return fmt.Errorf("backup current homepage: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect current homepage: %w", err)
	}
	if err := os.Rename(tempPath, g.cfg.Site.Output); err != nil {
		if hadOutput {
			_ = os.Rename(backupPath, g.cfg.Site.Output)
		}
		return fmt.Errorf("replace homepage: %w", err)
	}
	cleanupTemp = false
	if hadOutput {
		_ = os.Remove(backupPath)
	}
	return nil
}

func validateRenderedPage(page []byte) error {
	if len(page) < 1024 {
		return errors.New("rendered homepage is unexpectedly small")
	}
	text := string(page)
	required := []string{
		"<!DOCTYPE html>",
		"data-generated-at=",
		"class=\"top-news-grid\"",
		"class=\"work-section\"",
		"class=\"industry-section\"",
		"class=\"statistics-section\"",
		"data-stats-news",
	}
	for _, marker := range required {
		if !strings.Contains(text, marker) {
			return fmt.Errorf("rendered homepage is missing required marker %q", marker)
		}
	}
	return nil
}
