package generator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"portal-static/internal/adapters/caam/model"
)

// GenerateArticleRelated resolves all affected columns on the server before
// publishing the detail and refreshing its list and main-page references.
func (g *SiteGenerator) GenerateArticleRelated(ctx context.Context, articleID int64) (ArticleResult, error) {
	target, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return ArticleResult{}, err
	}
	if target != g {
		return target.GenerateArticleRelated(runCtx, articleID)
	}
	ctx = runCtx
	started := time.Now()
	lock, err := g.acquireRelatedRefreshLock()
	if err != nil {
		return ArticleResult{}, err
	}
	defer lock.release()
	if err := g.prepareDist(); err != nil {
		return ArticleResult{}, fmt.Errorf("prepare related article output: %w", err)
	}

	columns, err := g.resolveArticleRefreshColumns(ctx, articleID)
	if err != nil {
		return ArticleResult{}, fmt.Errorf("load current article columns: %w", err)
	}

	result, err := g.pages.GenerateArticle(ctx, articleID)
	if err != nil {
		return ArticleResult{}, fmt.Errorf("generate related article detail: %w", err)
	}
	refresh, err := g.refreshRelatedReferences(ctx, columns)
	if err != nil {
		return ArticleResult{}, err
	}
	result.DurationSeconds = elapsedSeconds(started)
	result.GeneratedFiles += refresh.generatedFiles
	result.GeneratedDetails += refresh.generatedDetails
	result.GeneratedLists += refresh.generatedLists
	result.RefreshedColumnIDs = refresh.columnIDs
	result.RefreshedPages = refresh.pages
	if err := g.clearArticleRefreshColumns(articleID); err != nil {
		return ArticleResult{}, fmt.Errorf("clear pending article columns: %w", err)
	}
	g.logger.Info("文章关联静态化成功", "article_id", articleID, "columns", refresh.columnIDs, "pages", refresh.pages, "generated_files", result.GeneratedFiles)
	return result, nil
}

// DeleteArticleRelated requires the database record to be unavailable for
// publication. It removes references first and deletes the detail last so a
// failed refresh cannot leave existing list links pointing at a missing file.
func (g *SiteGenerator) DeleteArticleRelated(ctx context.Context, articleID int64) (DeleteArticleResult, error) {
	target, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return DeleteArticleResult{}, err
	}
	if target != g {
		return target.DeleteArticleRelated(runCtx, articleID)
	}
	ctx = runCtx
	started := time.Now()
	lock, err := g.acquireRelatedRefreshLock()
	if err != nil {
		return DeleteArticleResult{}, err
	}
	defer lock.release()
	if err := g.prepareDist(); err != nil {
		return DeleteArticleResult{}, fmt.Errorf("prepare related article output: %w", err)
	}

	if _, err := g.homeSource.FetchArticleColumns(ctx, articleID); err == nil {
		return DeleteArticleResult{}, ErrArticleStillPublished
	} else if !errors.Is(err, ErrArticleNotPublished) {
		return DeleteArticleResult{}, fmt.Errorf("check article publication state: %w", err)
	}
	columns, err := g.resolveArticleRefreshColumns(ctx, articleID)
	if err != nil {
		return DeleteArticleResult{}, fmt.Errorf("load previous article columns: %w", err)
	}
	refresh, err := g.refreshRelatedReferences(ctx, columns)
	if err != nil {
		return DeleteArticleResult{}, err
	}
	result, err := g.pages.DeleteArticle(ctx, articleID)
	if err != nil {
		return DeleteArticleResult{}, fmt.Errorf("delete related article detail: %w", err)
	}
	result.DurationSeconds = elapsedSeconds(started)
	result.GeneratedFiles = refresh.generatedFiles
	result.GeneratedDetails = refresh.generatedDetails
	result.GeneratedLists = refresh.generatedLists
	result.RefreshedColumnIDs = refresh.columnIDs
	result.RefreshedPages = refresh.pages
	if err := g.clearArticleRefreshColumns(articleID); err != nil {
		return DeleteArticleResult{}, fmt.Errorf("clear pending article columns: %w", err)
	}
	g.logger.Info("文章关联静态页删除成功", "article_id", articleID, "columns", refresh.columnIDs, "pages", refresh.pages, "deleted", result.Deleted)
	return result, nil
}

func (g *SiteGenerator) acquireRelatedRefreshLock() (*fileLock, error) {
	target := filepath.Clean(g.cfg.Site.DistRoot)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, fmt.Errorf("create related refresh lock parent: %w", err)
	}
	return acquireFileLock(target+".lock", g.pages.staleAfter)
}

func mergeColumns(groups ...[]model.Column) []model.Column {
	byID := make(map[int64]model.Column)
	for _, columns := range groups {
		for _, column := range columns {
			if column.ID <= 0 {
				continue
			}
			byID[column.ID] = column
		}
	}
	ids := make([]int64, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	columns := make([]model.Column, 0, len(ids))
	for _, id := range ids {
		columns = append(columns, byID[id])
	}
	return columns
}

type relatedRefreshResult struct {
	generatedFiles   int
	generatedDetails int
	generatedLists   int
	columnIDs        []int64
	pages            []string
}

func (g *SiteGenerator) refreshRelatedReferences(ctx context.Context, columns []model.Column) (relatedRefreshResult, error) {
	columns = mergeColumns(columns)
	result := relatedRefreshResult{columnIDs: make([]int64, 0, len(columns))}
	for _, column := range columns {
		list, err := g.pages.GenerateList(ctx, column.ID)
		if err != nil {
			return relatedRefreshResult{}, fmt.Errorf("refresh related list %d: %w", column.ID, err)
		}
		result.columnIDs = append(result.columnIDs, column.ID)
		result.generatedFiles += list.GeneratedFiles
		result.generatedLists += list.GeneratedLists
	}

	pageNames := g.relatedPageNames(columns)
	pageGenerator := *g
	pageGenerator.skipScaffoldCopy = true
	result.pages = make([]string, 0, len(pageNames))
	for _, pageName := range pageNames {
		page, err := pageGenerator.GeneratePage(ctx, pageName)
		if err != nil {
			return relatedRefreshResult{}, fmt.Errorf("refresh related page %s: %w", pageName, err)
		}
		result.pages = append(result.pages, pageName)
		result.generatedFiles += page.GeneratedFiles
		result.generatedDetails += page.GeneratedDetails
		result.generatedLists += page.GeneratedLists
	}
	return result, nil
}

func (g *SiteGenerator) relatedPageNames(columns []model.Column) []string {
	affected := make(map[string]struct{})
	columnNames := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		if name := strings.TrimSpace(column.Name); name != "" {
			columnNames[name] = struct{}{}
		}
	}
	mark := func(page string, names ...string) {
		for _, name := range names {
			if _, exists := columnNames[strings.TrimSpace(name)]; exists {
				affected[page] = struct{}{}
				return
			}
		}
	}
	homeNames := make([]string, 0, len(g.cfg.ContentSlots()))
	for _, slot := range g.cfg.ContentSlots() {
		homeNames = append(homeNames, slot.Name)
	}
	mark("home", homeNames...)
	aboutNames := make([]string, 0, len(g.aboutSpecs()))
	for _, spec := range g.aboutSpecs() {
		aboutNames = append(aboutNames, spec.name)
	}
	mark("about", aboutNames...)
	w := g.cfg.WorkPage
	mark("work", w.Headline, w.Association, w.Branch, w.Industry, w.International, w.Expo, w.Platform)
	s := g.cfg.StatsPage
	mark("stats", s.DomesticReports, s.OverseasReports, s.ProductionReports, s.ImportExportReports,
		s.DomesticChart, s.OverseasChart, s.ProductionChart, s.ImportExportChart)
	m := g.cfg.MembersPage
	mark("members", m.Work, m.Style, m.Policy, m.Charter, m.Fees, m.President, m.VicePresident,
		m.BranchIntro, m.BranchRules, m.BranchService, m.BranchRoster, m.Management,
		m.ExecutiveDirector, m.Director, m.Delegate, m.Member)
	p := g.cfg.PartyPage
	mark("party", p.Work, p.Study, p.News, p.Carousel)

	ordered := make([]string, 0, len(affected))
	for _, name := range []string{"home", "about", "work", "stats", "members", "party"} {
		if _, exists := affected[name]; exists {
			ordered = append(ordered, name)
		}
	}
	return ordered
}
