package generator

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"portal-static/internal/adapters/caam/model"
)

const (
	mappingBatchSize = 2000
	listBatchSize    = 500
	detailBatchSize  = 100
	batchAttempts    = 3
	batchTimeout     = 30 * time.Second
)

type contentSnapshot struct {
	columns              []model.Column
	columnsByID          map[int64]model.Column
	articleIDs           []int64
	columnIDsByArticle   map[int64][]int64
	listArticlesByColumn map[int64][]model.Article
}

func (g *PageGenerator) loadContentSnapshot(ctx context.Context) (*contentSnapshot, error) {
	columns, err := retryRead(ctx, g.logger, "read columns", func(attemptCtx context.Context) ([]model.Column, error) {
		return g.source.FetchColumns(attemptCtx)
	})
	if err != nil {
		return nil, err
	}

	uniqueColumns := make([]model.Column, 0, len(columns))
	columnsByID := make(map[int64]model.Column, len(columns))
	for _, column := range columns {
		if column.ID <= 0 {
			continue
		}
		if _, exists := columnsByID[column.ID]; exists {
			continue
		}
		columnsByID[column.ID] = column
		uniqueColumns = append(uniqueColumns, column)
	}

	columnIDsByArticle := make(map[int64][]int64)
	seenPair := make(map[[2]int64]struct{})
	afterID := int64(0)
	mappingCount := 0
	for {
		mappings, batchErr := retryRead(ctx, g.logger, "read article-column mappings", func(attemptCtx context.Context) ([]model.ArticleColumnMapping, error) {
			return g.source.FetchArticleColumnMappingsBatch(attemptCtx, afterID, mappingBatchSize)
		})
		if batchErr != nil {
			return nil, batchErr
		}
		if len(mappings) == 0 {
			break
		}
		for _, mapping := range mappings {
			if mapping.ID > afterID {
				afterID = mapping.ID
			}
			if mapping.ArticleID <= 0 || mapping.ColumnID <= 0 {
				continue
			}
			if _, exists := columnsByID[mapping.ColumnID]; !exists {
				continue
			}
			pair := [2]int64{mapping.ArticleID, mapping.ColumnID}
			if _, exists := seenPair[pair]; exists {
				continue
			}
			seenPair[pair] = struct{}{}
			columnIDsByArticle[mapping.ArticleID] = append(columnIDsByArticle[mapping.ArticleID], mapping.ColumnID)
			mappingCount++
		}
		reportProgress(ctx, Progress{Stage: "读取栏目关系", Processed: mappingCount})
		g.logger.Info("批量读取栏目关系", "processed", mappingCount, "cursor", afterID)
		if len(mappings) < mappingBatchSize {
			break
		}
	}

	articleIDs := make([]int64, 0, len(columnIDsByArticle))
	for articleID, columnIDs := range columnIDsByArticle {
		sort.Slice(columnIDs, func(i, j int) bool { return columnIDs[i] < columnIDs[j] })
		columnIDsByArticle[articleID] = columnIDs
		articleIDs = append(articleIDs, articleID)
	}
	sort.Slice(articleIDs, func(i, j int) bool { return articleIDs[i] < articleIDs[j] })

	listArticlesByColumn := make(map[int64][]model.Article, len(uniqueColumns))
	for _, column := range uniqueColumns {
		listArticlesByColumn[column.ID] = nil
	}
	type listReadBatch struct {
		ids []int64
	}
	type listReadResult struct {
		requested int
		articles  []model.Article
		err       error
	}
	workCtx, cancelReads := context.WithCancel(ctx)
	defer cancelReads()
	readJobs := make(chan listReadBatch)
	readResults := make(chan listReadResult)
	var readWG sync.WaitGroup
	readWorker := func() {
		defer readWG.Done()
		for batch := range readJobs {
			articles, batchErr := retryRead(workCtx, g.logger, "read list article batch", func(attemptCtx context.Context) ([]model.Article, error) {
				return g.source.FetchListArticlesByIDs(attemptCtx, batch.ids)
			})
			select {
			case readResults <- listReadResult{requested: len(batch.ids), articles: articles, err: batchErr}:
			case <-workCtx.Done():
				return
			}
			if batchErr != nil {
				return
			}
		}
	}
	readWorkers := 4
	if len(articleIDs) < readWorkers {
		readWorkers = len(articleIDs)
	}
	for index := 0; index < readWorkers; index++ {
		readWG.Add(1)
		go readWorker()
	}
	go func() {
		defer close(readJobs)
		for start := 0; start < len(articleIDs); start += listBatchSize {
			end := start + listBatchSize
			if end > len(articleIDs) {
				end = len(articleIDs)
			}
			ids := append([]int64(nil), articleIDs[start:end]...)
			select {
			case readJobs <- listReadBatch{ids: ids}:
			case <-workCtx.Done():
				return
			}
		}
	}()
	go func() {
		readWG.Wait()
		close(readResults)
	}()
	processedArticles := 0
	for batch := range readResults {
		if batch.err != nil {
			cancelReads()
			return nil, batch.err
		}
		for _, article := range batch.articles {
			articleID, parseErr := strconv.ParseInt(strings.TrimSpace(article.ID), 10, 64)
			if parseErr != nil || articleID <= 0 {
				continue
			}
			for _, columnID := range columnIDsByArticle[articleID] {
				item := article
				item.ColumnID = columnID
				listArticlesByColumn[columnID] = append(listArticlesByColumn[columnID], item)
			}
		}
		processedArticles += batch.requested
		reportProgress(ctx, Progress{Stage: "读取列表文章", Processed: processedArticles, Total: len(articleIDs)})
		g.logger.Info("批量读取列表文章", "processed", processedArticles, "total", len(articleIDs))
	}

	for columnID := range listArticlesByColumn {
		articles := listArticlesByColumn[columnID]
		sort.SliceStable(articles, func(i, j int) bool {
			if articles[i].IsTop != articles[j].IsTop {
				return articles[i].IsTop
			}
			if !articles[i].PublishTime.Equal(articles[j].PublishTime) {
				return articles[i].PublishTime.After(articles[j].PublishTime)
			}
			left, _ := strconv.ParseInt(articles[i].ID, 10, 64)
			right, _ := strconv.ParseInt(articles[j].ID, 10, 64)
			return left > right
		})
		listArticlesByColumn[columnID] = articles
	}

	return &contentSnapshot{
		columns: uniqueColumns, columnsByID: columnsByID, articleIDs: articleIDs,
		columnIDsByArticle: columnIDsByArticle, listArticlesByColumn: listArticlesByColumn,
	}, nil
}

func retryRead[T any](ctx context.Context, logger interface {
	Warn(string, ...any)
}, operation string, read func(context.Context) (T, error)) (T, error) {
	var zero T
	var lastErr error
	for attempt := 1; attempt <= batchAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, batchTimeout)
		value, err := read(attemptCtx)
		cancel()
		if err == nil {
			return value, nil
		}
		lastErr = err
		if attempt == batchAttempts || !isTransientReadError(err) || ctx.Err() != nil {
			break
		}
		logger.Warn("数据库批次读取失败，准备重试", "operation", operation, "attempt", attempt, "error", err)
		delay := time.Duration(attempt) * 250 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
	return zero, fmt.Errorf("%s: %w", operation, lastErr)
}

func isTransientReadError(err error) bool {
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{"invalid connection", "unexpected eof", "broken pipe", "connection reset", "lost connection", "deadlock", "lock wait timeout"} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}
