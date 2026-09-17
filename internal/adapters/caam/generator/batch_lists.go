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
	"sync"
	"time"

	"portal-static/internal/adapters/caam/model"
)

func (g *PageGenerator) GenerateAllLists(ctx context.Context) (AllListsResult, error) {
	started := time.Now()
	snapshot, err := g.loadContentSnapshot(ctx)
	if err != nil {
		return AllListsResult{}, err
	}
	result, err := g.generateAllListsWithSnapshot(ctx, snapshot)
	if err == nil {
		result.DurationSeconds = math.Round(time.Since(started).Seconds()*1000) / 1000
	}
	return result, err
}

func (g *PageGenerator) generateAllListsWithSnapshot(ctx context.Context, snapshot *contentSnapshot) (AllListsResult, error) {
	started := time.Now()
	listRoot := filepath.Join(g.cfg.Site.OutputRoot, "list")
	listParent := filepath.Dir(listRoot)
	if err := os.MkdirAll(listParent, 0o755); err != nil {
		return AllListsResult{}, fmt.Errorf("create list output parent: %w", err)
	}
	batchLock, err := acquireFileLock(listRoot+".lock", g.staleAfter)
	if err != nil {
		return AllListsResult{}, err
	}
	defer batchLock.release()

	footerLinks, err := g.fetchFooterLinks(ctx)
	if err != nil {
		return AllListsResult{}, err
	}
	tpl, err := template.ParseFiles(g.cfg.Site.ListTemplate)
	if err != nil {
		return AllListsResult{}, fmt.Errorf("parse list template: %w", err)
	}
	staging, err := os.MkdirTemp(listParent, ".all-lists-*")
	if err != nil {
		return AllListsResult{}, fmt.Errorf("create all-list staging directory: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(staging)
		}
	}()

	type listWork struct {
		index  int
		column model.Column
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan listWork)
	results := make([]ListResult, len(snapshot.columns))
	var wg sync.WaitGroup
	var stateMu sync.Mutex
	var firstErr error
	completed := 0
	generatedFiles := 0
	worker := func() {
		defer wg.Done()
		for job := range jobs {
			if workCtx.Err() != nil {
				return
			}
			g.logger.Info("生成栏目列表", "column_id", job.column.ID, "column", job.column.Name)
			target := filepath.Join(staging, strconv.FormatInt(job.column.ID, 10))
			result, renderErr := g.renderBatchListDirectory(
				workCtx, target, job.column, snapshot.listArticlesByColumn[job.column.ID], footerLinks, tpl,
			)
			stateMu.Lock()
			if renderErr != nil && firstErr == nil {
				firstErr = fmt.Errorf("generate column %d list: %w", job.column.ID, renderErr)
				cancel()
			}
			if renderErr == nil {
				result.Output = filepath.Join(listRoot, strconv.FormatInt(job.column.ID, 10))
				results[job.index] = result
				completed++
				generatedFiles += result.TotalPages
				reportProgress(ctx, Progress{
					Stage: "生成栏目列表", Processed: completed, Total: len(snapshot.columns),
					CurrentColumnID: job.column.ID, GeneratedFiles: generatedFiles,
				})
			}
			stateMu.Unlock()
		}
	}
	workerCount := 4
	if len(snapshot.columns) < workerCount {
		workerCount = len(snapshot.columns)
	}
	for index := 0; index < workerCount; index++ {
		wg.Add(1)
		go worker()
	}
feed:
	for index, column := range snapshot.columns {
		select {
		case <-workCtx.Done():
			break feed
		case jobs <- listWork{index: index, column: column}:
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return AllListsResult{}, firstErr
	}
	if err := ctx.Err(); err != nil {
		return AllListsResult{}, err
	}
	if completed != len(snapshot.columns) {
		return AllListsResult{}, fmt.Errorf("generated %d of %d columns", completed, len(snapshot.columns))
	}
	if err := replaceDirectory(staging, listRoot); err != nil {
		return AllListsResult{}, fmt.Errorf("publish all lists: %w", err)
	}
	cleanup = false

	totalItems := 0
	totalPages := 0
	for _, result := range results {
		totalItems += result.TotalItems
		totalPages += result.TotalPages
	}
	result := AllListsResult{
		GeneratedAt:     time.Now().In(g.location),
		DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000,
		GeneratedFiles:  totalPages,
		GeneratedLists:  totalPages,
		Generated:       len(snapshot.columns), TotalItems: totalItems, TotalPages: totalPages,
		Output: listRoot, Lists: results,
	}
	g.logger.Info("全部栏目列表批量生成成功",
		"generated_columns", result.Generated, "total_items", result.TotalItems,
		"total_pages", result.TotalPages, "duration_seconds", result.DurationSeconds, "output", result.Output,
	)
	return result, nil
}

func (g *PageGenerator) renderBatchListDirectory(
	ctx context.Context,
	target string,
	column model.Column,
	articles []model.Article,
	footerLinks []LinkGroupView,
	tpl *template.Template,
) (ListResult, error) {
	started := time.Now()
	totalPages := int(math.Ceil(float64(len(articles)) / float64(g.cfg.Site.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return ListResult{}, fmt.Errorf("create column list directory: %w", err)
	}
	title := g.columnTitle(column)
	generatedAt := time.Now().In(g.location)
	for page := 1; page <= totalPages; page++ {
		if err := ctx.Err(); err != nil {
			return ListResult{}, err
		}
		start := (page - 1) * g.cfg.Site.PageSize
		end := start + g.cfg.Site.PageSize
		if start > len(articles) {
			start = len(articles)
		}
		if end > len(articles) {
			end = len(articles)
		}
		items := make([]InnerArticleView, 0, end-start)
		for _, article := range articles[start:end] {
			item := g.innerArticle(article, column.ID, "../../")
			if !item.ExternalLink {
				item.Href = withListOrigin(item.Href, column.ID, title, page)
			}
			items = append(items, item)
		}
		data := ListPageData{
			GeneratedAt: generatedAt.Format(time.RFC3339), RootPrefix: "../../",
			ColumnID: column.ID, Title: title, Items: items, Page: page,
			PageSize: g.cfg.Site.PageSize, TotalItems: len(articles), TotalPages: totalPages,
			Pagination: buildPagination(page, totalPages), FooterLinks: footerLinks,
		}
		if page > 1 {
			data.Previous = strconv.Itoa(page-1) + ".html"
		}
		if page < totalPages {
			data.Next = strconv.Itoa(page+1) + ".html"
		}
		var rendered bytes.Buffer
		if err := tpl.Execute(&rendered, data); err != nil {
			return ListResult{}, fmt.Errorf("render list page %d: %w", page, err)
		}
		if err := validateInnerPage(rendered.Bytes(), "class=\"full-news-list\""); err != nil {
			return ListResult{}, fmt.Errorf("validate list page %d: %w", page, err)
		}
		if err := os.WriteFile(filepath.Join(target, strconv.Itoa(page)+".html"), rendered.Bytes(), 0o644); err != nil {
			return ListResult{}, fmt.Errorf("write list page %d: %w", page, err)
		}
		reportProgress(ctx, Progress{
			Stage: "生成栏目分页", Processed: page, Total: totalPages,
			CurrentColumnID: column.ID, GeneratedFiles: page,
		})
	}
	return ListResult{
		GeneratedAt: generatedAt, DurationSeconds: elapsedSeconds(started), GeneratedFiles: totalPages,
		ColumnID: column.ID, TotalItems: len(articles),
		TotalPages: totalPages, PageSize: g.cfg.Site.PageSize, Output: target,
	}, nil
}
