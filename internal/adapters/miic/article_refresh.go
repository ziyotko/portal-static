package miic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"portal-static/internal/adapters/miic/model"
	"portal-static/internal/adapters/miic/repository"
)

// GenerateArticleRelated publishes the detail first, then refreshes every
// automatically resolved current or previous column and its related main page.
func (g *Generator) GenerateArticleRelated(ctx context.Context, articleID int64) (ArticleResult, error) {
	started := time.Now()
	root, err := g.outputRoot(ctx)
	if err != nil {
		return ArticleResult{}, err
	}
	lock, err := g.acquireRelatedRefreshLock(root)
	if err != nil {
		return ArticleResult{}, err
	}
	defer lock.release()

	columns, err := g.resolveArticleRefreshColumns(ctx, root, articleID)
	if err != nil {
		return ArticleResult{}, err
	}
	if err := copyRelatedScaffold(g.cfg.Site.SourceRoot, root); err != nil {
		return ArticleResult{}, fmt.Errorf("prepare related article output: %w", err)
	}

	column, article, err := g.source.FetchArticle(ctx, articleID)
	if err != nil {
		return ArticleResult{}, fmt.Errorf("generate related article detail: %w", err)
	}
	if _, external := safeExternal(article.URL); external {
		return ArticleResult{}, errors.New("external articles do not have local detail pages")
	}
	if !model.HasStaticDetail(article.Type) {
		return ArticleResult{}, repository.ErrArticleNotPublished
	}
	if preferred, _, contextErr := g.categoryContext(ctx); contextErr != nil {
		return ArticleResult{}, fmt.Errorf("generate related article detail: %w", contextErr)
	} else if selected, exists := preferred[articleID]; exists {
		column = selected
	}
	path, err := g.renderArticle(root, column, article)
	if err != nil {
		return ArticleResult{}, fmt.Errorf("generate related article detail: %w", err)
	}

	refresh, err := g.refreshRelatedReferences(ctx, root, columns)
	if err != nil {
		return ArticleResult{}, err
	}
	if err := clearArticleRefreshColumns(root, articleID); err != nil {
		return ArticleResult{}, err
	}
	result := ArticleResult{
		GeneratedAt:        time.Now().In(g.location),
		DurationSeconds:    roundDuration(started),
		GeneratedFiles:     1 + refresh.generatedFiles,
		GeneratedDetails:   1 + refresh.generatedDetails,
		GeneratedLists:     refresh.generatedLists,
		ArticleID:          articleID,
		ColumnID:           column.ID,
		Output:             path,
		RefreshedColumnIDs: refresh.columnIDs,
		RefreshedPages:     refresh.pages,
	}
	g.logger.Info("文章关联静态化成功", "article_id", articleID, "columns", refresh.columnIDs, "pages", refresh.pages, "generated_files", result.GeneratedFiles)
	return result, nil
}

// DeleteArticleRelated requires the database record to be unavailable for
// publication. References are refreshed before the detail is deleted so a
// failed refresh cannot leave a live list link pointing at a missing file.
func (g *Generator) DeleteArticleRelated(ctx context.Context, articleID int64) (DeleteArticleResult, error) {
	started := time.Now()
	root, err := g.outputRoot(ctx)
	if err != nil {
		return DeleteArticleResult{}, err
	}
	lock, err := g.acquireRelatedRefreshLock(root)
	if err != nil {
		return DeleteArticleResult{}, err
	}
	defer lock.release()

	if _, err := g.source.FetchArticleColumns(ctx, articleID); err == nil {
		return DeleteArticleResult{}, ErrArticleStillPublished
	} else if !errors.Is(err, repository.ErrArticleNotPublished) {
		return DeleteArticleResult{}, fmt.Errorf("check article publication state: %w", err)
	}
	columns, err := g.resolveArticleRefreshColumns(ctx, root, articleID)
	if err != nil {
		return DeleteArticleResult{}, err
	}
	if err := copyRelatedScaffold(g.cfg.Site.SourceRoot, root); err != nil {
		return DeleteArticleResult{}, fmt.Errorf("prepare related article output: %w", err)
	}
	refresh, err := g.refreshRelatedReferences(ctx, root, columns)
	if err != nil {
		return DeleteArticleResult{}, err
	}
	result, err := g.DeleteArticle(ctx, articleID)
	if err != nil {
		return DeleteArticleResult{}, fmt.Errorf("delete related article detail: %w", err)
	}
	if err := clearArticleRefreshColumns(root, articleID); err != nil {
		return DeleteArticleResult{}, err
	}
	result.DurationSeconds = roundDuration(started)
	result.GeneratedFiles = refresh.generatedFiles
	result.GeneratedDetails = refresh.generatedDetails
	result.GeneratedLists = refresh.generatedLists
	result.RefreshedColumnIDs = refresh.columnIDs
	result.RefreshedPages = refresh.pages
	g.logger.Info("文章关联静态页删除成功", "article_id", articleID, "columns", refresh.columnIDs, "pages", refresh.pages, "deleted", result.Deleted)
	return result, nil
}

func (g *Generator) acquireRelatedRefreshLock(root string) (*fileLock, error) {
	if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
		return nil, fmt.Errorf("create related refresh lock parent: %w", err)
	}
	return acquireLock(filepath.Clean(root)+".lock", g.stale)
}

func mergeColumns(groups ...[]model.Column) []model.Column {
	byID := make(map[int64]model.Column)
	for _, columns := range groups {
		for _, column := range columns {
			if column.ID > 0 {
				byID[column.ID] = column
			}
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

func (g *Generator) refreshRelatedReferences(ctx context.Context, root string, columns []model.Column) (relatedRefreshResult, error) {
	columns = mergeColumns(columns)
	result := relatedRefreshResult{columnIDs: make([]int64, 0, len(columns))}
	for _, column := range columns {
		list, err := g.generateListAt(ctx, root, column.ID)
		if err != nil {
			return relatedRefreshResult{}, fmt.Errorf("refresh related list %d: %w", column.ID, err)
		}
		result.columnIDs = append(result.columnIDs, column.ID)
		result.generatedFiles += list.GeneratedFiles
		result.generatedLists += list.GeneratedLists
	}

	result.pages = relatedPageNames(columns)
	for _, pageName := range result.pages {
		if err := g.renderMainPageAt(ctx, root, pageName); err != nil {
			return relatedRefreshResult{}, fmt.Errorf("refresh related page %s: %w", pageName, err)
		}
		result.generatedFiles++
	}
	if err := g.writeGeneratedContent(ctx, root); err != nil {
		return relatedRefreshResult{}, fmt.Errorf("refresh related page data: %w", err)
	}
	result.generatedFiles++
	return result, nil
}

func (g *Generator) renderMainPageAt(ctx context.Context, root, normalized string) error {
	if normalized == "news" {
		return g.renderNews(ctx, root, false)
	}
	data, err := os.ReadFile(filepath.Join(g.cfg.Site.SourceRoot, normalized+".html"))
	if err != nil {
		return err
	}
	return publishFile(filepath.Join(root, normalized+".html"), grayscaleHTML(data, false))
}

func relatedPageNames(columns []model.Column) []string {
	affected := make(map[string]struct{})
	for _, column := range columns {
		if normalized, ok := NormalizePageName(strings.TrimSpace(column.PageName)); ok {
			affected[normalized] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(affected))
	for _, name := range []string{"news", "business", "platforms", "about"} {
		if _, exists := affected[name]; exists {
			ordered = append(ordered, name)
		}
	}
	return ordered
}
