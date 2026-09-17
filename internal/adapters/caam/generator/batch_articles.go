package generator

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"portal-static/internal/adapters/caam/model"
)

func (g *PageGenerator) GenerateAllArticles(ctx context.Context) (AllArticlesResult, error) {
	started := time.Now()
	snapshot, err := g.loadContentSnapshot(ctx)
	if err != nil {
		return AllArticlesResult{}, err
	}
	result, err := g.generateAllArticlesWithSnapshot(ctx, snapshot)
	if err == nil {
		result.DurationSeconds = math.Round(time.Since(started).Seconds()*1000) / 1000
	}
	return result, err
}

func (g *PageGenerator) generateAllArticlesWithSnapshot(ctx context.Context, snapshot *contentSnapshot) (AllArticlesResult, error) {
	started := time.Now()
	articleRoot := filepath.Join(g.cfg.Site.OutputRoot, "article")
	articleParent := filepath.Dir(articleRoot)
	if err := os.MkdirAll(articleParent, 0o755); err != nil {
		return AllArticlesResult{}, fmt.Errorf("create article output parent: %w", err)
	}
	batchLock, err := acquireFileLock(articleRoot+".lock", g.staleAfter)
	if err != nil {
		return AllArticlesResult{}, err
	}
	defer batchLock.release()
	footerLinks, err := g.fetchFooterLinks(ctx)
	if err != nil {
		return AllArticlesResult{}, err
	}
	tpl, err := template.ParseFiles(g.cfg.Site.ArticleTemplate)
	if err != nil {
		return AllArticlesResult{}, fmt.Errorf("parse article template: %w", err)
	}
	staging, err := os.MkdirTemp(articleParent, ".all-articles-*")
	if err != nil {
		return AllArticlesResult{}, fmt.Errorf("create article staging directory: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(staging)
		}
	}()

	generated := 0
	skippedExternal := 0
	skippedData := 0
	skippedInvalid := 0
	for start := 0; start < len(snapshot.articleIDs); start += detailBatchSize * 4 {
		if err := ctx.Err(); err != nil {
			return AllArticlesResult{}, err
		}
		end := start + detailBatchSize*4
		if end > len(snapshot.articleIDs) {
			end = len(snapshot.articleIDs)
		}
		ids := snapshot.articleIDs[start:end]
		articles, batchErr := g.fetchDetailArticleWindow(ctx, ids)
		if batchErr != nil {
			return AllArticlesResult{}, batchErr
		}
		skippedData += len(ids) - len(articles)

		type articleWork struct {
			article  model.Article
			columnID int64
			column   model.Column
		}
		jobs := make(chan articleWork)
		workCtx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		var stateMu sync.Mutex
		var firstErr error
		batchGenerated := 0
		batchExternal := 0
		batchInvalid := 0
		worker := func() {
			defer wg.Done()
			for job := range jobs {
				if workCtx.Err() != nil {
					return
				}
				articleID, renderErr := g.renderBatchArticle(workCtx, staging, job.column, job.article, footerLinks, tpl)
				stateMu.Lock()
				if renderErr != nil && firstErr == nil {
					firstErr = renderErr
					cancel()
				}
				if renderErr == nil {
					batchGenerated++
					reportProgress(ctx, Progress{
						Stage: "生成文章详情", Processed: generated + batchGenerated,
						Total: len(snapshot.articleIDs), CurrentArticleID: articleID,
						GeneratedFiles: generated + batchGenerated,
					})
				}
				stateMu.Unlock()
			}
		}
		workerCount := 4
		if len(articles) < workerCount {
			workerCount = len(articles)
		}
		for index := 0; index < workerCount; index++ {
			wg.Add(1)
			go worker()
		}
	feed:
		for _, article := range articles {
			if article.Type != model.ArticleTypeContent && article.Type != model.ArticleTypeVideo {
				continue
			}
			articleID, parseErr := strconv.ParseInt(strings.TrimSpace(article.ID), 10, 64)
			columnIDs := snapshot.columnIDsByArticle[articleID]
			if parseErr != nil || articleID <= 0 || len(columnIDs) == 0 || article.PublishTime.IsZero() {
				batchInvalid++
				continue
			}
			if _, external, ok := safeHref(article.URL); ok && external {
				batchExternal++
				continue
			}
			columnID := columnIDs[0]
			column, exists := snapshot.columnsByID[columnID]
			if !exists {
				batchInvalid++
				continue
			}
			article.ColumnID = columnID
			select {
			case <-workCtx.Done():
				break feed
			case jobs <- articleWork{article: article, columnID: columnID, column: column}:
			}
		}
		close(jobs)
		wg.Wait()
		cancel()
		if firstErr != nil {
			return AllArticlesResult{}, firstErr
		}
		if err := ctx.Err(); err != nil {
			return AllArticlesResult{}, err
		}
		generated += batchGenerated
		skippedExternal += batchExternal
		skippedInvalid += batchInvalid
		reportProgress(ctx, Progress{
			Stage: "生成文章详情", Processed: end, Total: len(snapshot.articleIDs), GeneratedFiles: generated,
		})
		g.logger.Info("详情文章批次生成完成", "processed", end, "total", len(snapshot.articleIDs), "generated", generated)
	}

	if err := replaceDirectory(staging, articleRoot); err != nil {
		return AllArticlesResult{}, fmt.Errorf("publish all articles: %w", err)
	}
	cleanup = false
	result := AllArticlesResult{
		GeneratedAt:      time.Now().In(g.location),
		DurationSeconds:  math.Round(time.Since(started).Seconds()*1000) / 1000,
		GeneratedFiles:   generated,
		GeneratedDetails: generated,
		Generated:        generated, SkippedExternal: skippedExternal, SkippedData: skippedData, SkippedInvalid: skippedInvalid,
		Output: articleRoot,
	}
	g.logger.Info("全部文章详情批量生成成功",
		"generated_articles", result.Generated, "skipped_external", result.SkippedExternal,
		"skipped_invalid", result.SkippedInvalid, "duration_seconds", result.DurationSeconds, "output", result.Output,
	)
	return result, nil
}

func (g *PageGenerator) fetchDetailArticleWindow(ctx context.Context, ids []int64) ([]model.Article, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	chunkCount := (len(ids) + detailBatchSize - 1) / detailBatchSize
	chunks := make([][]model.Article, chunkCount)
	var wg sync.WaitGroup
	var errorMu sync.Mutex
	var firstErr error
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	for index := 0; index < chunkCount; index++ {
		start := index * detailBatchSize
		end := start + detailBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		batchIDs := append([]int64(nil), ids[start:end]...)
		wg.Add(1)
		go func(chunkIndex int) {
			defer wg.Done()
			articles, err := retryRead(workCtx, g.logger, "read detail article batch", func(attemptCtx context.Context) ([]model.Article, error) {
				return g.source.FetchDetailArticlesByIDs(attemptCtx, batchIDs)
			})
			if err != nil {
				errorMu.Lock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				errorMu.Unlock()
				return
			}
			chunks[chunkIndex] = articles
		}(index)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	articles := make([]model.Article, 0, len(ids))
	for _, chunk := range chunks {
		articles = append(articles, chunk...)
	}
	return articles, nil
}

func (g *PageGenerator) renderBatchArticle(
	ctx context.Context,
	articleRoot string,
	column model.Column,
	article model.Article,
	footerLinks []LinkGroupView,
	tpl *template.Template,
) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	articleID, err := strconv.ParseInt(strings.TrimSpace(article.ID), 10, 64)
	if err != nil || articleID <= 0 || article.PublishTime.IsZero() {
		return 0, fmt.Errorf("invalid article archive identity %q", article.ID)
	}
	published := article.PublishTime.In(g.location)
	output := filepath.Join(articleRoot, published.Format("2006"), published.Format("01"), article.ID+".html")
	data := ArticlePageData{
		GeneratedAt: time.Now().In(g.location).Format(time.RFC3339), RootPrefix: "../../../",
		ColumnID: column.ID, ColumnTitle: g.columnTitle(column),
		Article: g.innerArticle(article, column.ID, "../../../"),
		Content: g.sanitizeContent(article.Content), FooterLinks: footerLinks,
	}
	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, data); err != nil {
		return 0, fmt.Errorf("render article %d: %w", articleID, err)
	}
	if err := validateInnerPage(rendered.Bytes(), "class=\"article-body\""); err != nil {
		return 0, fmt.Errorf("validate article %d: %w", articleID, err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return 0, fmt.Errorf("create article %d directory: %w", articleID, err)
	}
	if err := os.WriteFile(output, rendered.Bytes(), 0o644); err != nil {
		return 0, fmt.Errorf("write article %d: %w", articleID, err)
	}
	return articleID, nil
}
