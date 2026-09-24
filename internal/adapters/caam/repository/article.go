package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
	"portal-static/internal/sources/portalcms"
)

var ErrTemplateNotUnique = errors.New("模板名称必须且只能匹配一个有效模板")
var ErrTemplateNotFound = errors.New("模板不存在")
var ErrColumnNotUnique = errors.New("栏目名称必须且只能匹配一个栏目")
var ErrColumnNotFound = errors.New("栏目不存在")
var ErrArticleNotPublished = errors.New("文章未发布或不属于有效栏目")

const databaseOperationTimeout = 30 * time.Second

func withDatabaseOperationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, databaseOperationTimeout)
}

type ArticleSource interface {
	ResolveColumnID(context.Context, config.SlotConfig) (int64, error)
	FetchByColumn(context.Context, config.SlotConfig) ([]model.Article, error)
	FetchStatisticsTitles(context.Context, string) ([]string, error)
	FetchMonthlyStatistics(context.Context, string, string) ([]model.Article, error)
	FetchLinksByColumn(context.Context, config.SlotConfig) ([]model.Article, error)
}

func (r *ArticleRepository) ResolveColumnID(ctx context.Context, slot config.SlotConfig) (int64, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	return r.resolveColumnByName(ctx, slot.Name)
}

// ResolveGlobalColumnID resolves a single active column by its display name.
// Unlike ResolveColumnID, it is not limited to the configured home template and is
// intended for the public single-list endpoint.
func (r *ArticleRepository) ResolveGlobalColumnID(ctx context.Context, name string) (int64, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	name = strings.TrimSpace(name)
	rows, err := r.db.QueryContext(ctx,
		"SELECT id FROM {{schema}}.`column` WHERE name = ? AND status = 1 ORDER BY id ASC LIMIT 2",
		name,
	)
	if err != nil {
		return 0, fmt.Errorf("查询栏目“%s”失败：%w", name, err)
	}
	defer rows.Close()
	ids := make([]int64, 0, 2)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("读取栏目“%s”失败：%w", name, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("遍历栏目“%s”失败：%w", name, err)
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("%w：“%s”", ErrColumnNotFound, name)
	}
	if len(ids) > 1 {
		return 0, fmt.Errorf("%w：“%s”，匹配到 %d 条记录", ErrColumnNotUnique, name, len(ids))
	}
	return ids[0], nil
}

func (r *ArticleRepository) FetchLinksByColumn(ctx context.Context, slot config.SlotConfig) ([]model.Article, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	columnID, err := r.resolveColumnByName(ctx, slot.Name)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT l.id, l.name, l.url, l.logo
FROM {{schema}}.link l
JOIN {{schema}}.`+"`column`"+` c ON c.id = l.column_id
WHERE l.status = 1
  AND l.template_id = ?
  AND c.id = ?
  AND c.status = 1
  AND c.template_id = l.template_id
ORDER BY l.sort ASC, l.id ASC
LIMIT ?`, r.templateID, columnID, slot.Limit)
	if err != nil {
		return nil, fmt.Errorf("query link column %q: %w", slot.Name, err)
	}
	defer rows.Close()

	links := make([]model.Article, 0, slot.Limit)
	for rows.Next() {
		var (
			id   int64
			name sql.NullString
			href sql.NullString
			logo sql.NullString
		)
		if err := rows.Scan(&id, &name, &href, &logo); err != nil {
			return nil, fmt.Errorf("scan link column %q: %w", slot.Name, err)
		}
		links = append(links, model.Article{
			ID:    fmt.Sprintf("%d", id),
			Type:  1,
			Title: name.String,
			URL:   href.String,
			Cover: logo.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate link column %q: %w", slot.Name, err)
	}
	return links, nil
}

type ArticleRepository struct {
	db         *portalcms.Store
	templateID int
}

func ResolveTemplateID(ctx context.Context, db *sql.DB, name string) (int, error) {
	store, err := portalcms.NewStore(db, "caam_portal", false)
	if err != nil {
		return 0, err
	}
	return ResolveTemplateIDWithStore(ctx, store, name)
}

func ResolveTemplateIDWithStore(ctx context.Context, store *portalcms.Store, name string) (int, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	name = strings.TrimSpace(name)
	rows, err := store.QueryContext(ctx, `
SELECT id
FROM {{schema}}.template
WHERE name = ?
  AND type = 'home'
  AND status = 1
ORDER BY id ASC
LIMIT 2`, name)
	if err != nil {
		return 0, fmt.Errorf("查询有效模板“%s”失败：%w", name, err)
	}
	defer rows.Close()
	ids := make([]int, 0, 2)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("读取有效模板“%s”失败：%w", name, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("遍历有效模板“%s”失败：%w", name, err)
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("%w：“%s”", ErrTemplateNotFound, name)
	}
	if len(ids) > 1 {
		return 0, fmt.Errorf("%w：“%s”，匹配到 %d 条记录", ErrTemplateNotUnique, name, len(ids))
	}
	return ids[0], nil
}

func NewArticleRepository(db *sql.DB, templateID int) *ArticleRepository {
	store, err := portalcms.NewStore(db, "caam_portal", false)
	if err != nil {
		panic(err)
	}
	return NewArticleRepositoryWithStore(store, templateID)
}

func NewArticleRepositoryWithStore(store *portalcms.Store, templateID int) *ArticleRepository {
	return &ArticleRepository{db: store, templateID: templateID}
}

// FetchPublishedByColumnName returns all publishable articles attached to the
// requested display name. The CMS intentionally has a few same-name columns;
// those article sets are merged and each article keeps its original column ID.
func (r *ArticleRepository) FetchPublishedByColumnName(ctx context.Context, name string) (model.Column, []model.Article, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	columns, err := r.resolveColumnsByName(ctx, name)
	if err != nil {
		return model.Column{}, nil, err
	}
	return r.fetchPublishedColumns(ctx, strings.TrimSpace(name), columns)
}

// FetchPublishedByColumnNameLimit is used by navigation pages. It reads only
// the newest records that can actually be displayed instead of loading every
// article body in a large archive column.
func (r *ArticleRepository) FetchPublishedByColumnNameLimit(ctx context.Context, name string, types []int, limit int) (model.Column, []model.Article, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	columns, err := r.resolveColumnsByName(ctx, name)
	if err != nil {
		return model.Column{}, nil, err
	}
	name = strings.TrimSpace(name)
	if limit <= 0 {
		return r.fetchPublishedColumns(ctx, name, columns)
	}
	if len(types) == 0 {
		types = []int{model.ArticleTypeContent, model.ArticleTypeVideo}
	}

	columnPlaceholders := make([]string, len(columns))
	typePlaceholders := make([]string, len(types))
	args := make([]any, 0, len(columns)+len(types)+1)
	for index, column := range columns {
		columnPlaceholders[index] = "?"
		args = append(args, column.ID)
	}
	for index, articleType := range types {
		typePlaceholders[index] = "?"
		args = append(args, articleType)
	}
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, `
SELECT selected.column_id, a.id, a.type, a.title, a.summary, a.content, a.cover,
       a.author, a.source, a.is_bold, a.default_color, a.url, a.publish_time
FROM (
    SELECT acp.column_id, candidate.id
    FROM {{schema}}.article candidate
    INNER JOIN {{schema}}.article_column_publish acp
      ON acp.column_id IN (`+strings.Join(columnPlaceholders, ",")+`)
     AND acp.article_id = candidate.id
    INNER JOIN {{schema}}.`+"`column`"+` c
      ON c.id = acp.column_id
     AND c.status = 1
     AND c.template_id = acp.template_id
    WHERE candidate.status = 1
      AND candidate.audit_status = 2
      AND candidate.type IN (`+strings.Join(typePlaceholders, ",")+`)
      AND candidate.type IN (1, 2, 3)
    ORDER BY candidate.is_top DESC, candidate.publish_time DESC, candidate.id DESC
    LIMIT ?
) selected
INNER JOIN {{schema}}.article a ON a.id = selected.id
ORDER BY a.is_top DESC, a.publish_time DESC, a.id DESC`, args...)
	if err != nil {
		return model.Column{}, nil, fmt.Errorf("query limited global column name %q: %w", name, err)
	}
	defer rows.Close()

	articles := make([]model.Article, 0, limit)
	seen := make(map[string]struct{}, limit)
	for rows.Next() {
		article, scanErr := scanNamedColumnDetailArticle(rows)
		if scanErr != nil {
			return model.Column{}, nil, fmt.Errorf("scan limited global column name %q article: %w", name, scanErr)
		}
		if _, exists := seen[article.ID]; exists {
			continue
		}
		seen[article.ID] = struct{}{}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return model.Column{}, nil, fmt.Errorf("iterate limited global column name %q articles: %w", name, err)
	}
	column := columns[0]
	column.Name = name
	if len(articles) > 0 {
		column.ID = articles[0].ColumnID
	}
	return column, articles, nil
}

func (r *ArticleRepository) resolveColumnsByName(ctx context.Context, name string) ([]model.Column, error) {
	name = strings.TrimSpace(name)
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, name FROM {{schema}}.`column` WHERE name = ? AND status = 1 ORDER BY id ASC",
		name,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve global column %q: %w", name, err)
	}
	columns := make([]model.Column, 0, 2)
	for rows.Next() {
		var column model.Column
		var columnName sql.NullString
		if err := rows.Scan(&column.ID, &columnName); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan global column %q: %w", name, err)
		}
		column.Name = columnName.String
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate global column %q: %w", name, err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close global column %q: %w", name, err)
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrColumnNotFound, name)
	}
	return columns, nil
}

func (r *ArticleRepository) fetchPublishedColumns(ctx context.Context, name string, columns []model.Column) (model.Column, []model.Article, error) {
	placeholders := make([]string, len(columns))
	args := make([]any, 0, len(columns))
	for index, column := range columns {
		placeholders[index] = "?"
		args = append(args, column.ID)
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT acp.column_id, a.id, a.type, a.title, a.summary, a.content, a.cover,
       a.author, a.source, a.is_bold, a.default_color,
       a.url, a.publish_time
FROM {{schema}}.article_column_publish acp
INNER JOIN {{schema}}.article a ON a.id = acp.article_id
INNER JOIN {{schema}}.`+"`column`"+` c
        ON c.id = acp.column_id
       AND c.status = 1
       AND c.template_id = acp.template_id
WHERE acp.column_id IN (`+strings.Join(placeholders, ",")+`)
  AND a.status = 1
  AND a.audit_status = 2
  AND a.type IN (1, 2, 3)
ORDER BY a.is_top DESC, a.publish_time DESC, a.id DESC`, args...)
	if err != nil {
		return model.Column{}, nil, fmt.Errorf("query global column name %q: %w", name, err)
	}
	defer rows.Close()

	articles := make([]model.Article, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		article, scanErr := scanNamedColumnDetailArticle(rows)
		if scanErr != nil {
			return model.Column{}, nil, fmt.Errorf("scan global column name %q article: %w", name, scanErr)
		}
		if _, exists := seen[article.ID]; exists {
			continue
		}
		seen[article.ID] = struct{}{}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return model.Column{}, nil, fmt.Errorf("iterate global column name %q articles: %w", name, err)
	}
	column := columns[0]
	column.Name = name
	return column, articles, nil
}

// FetchPublishedByColumnID resolves a column globally by primary key. Some CMS
// pages intentionally reuse a display name for a report list and chart data.
func (r *ArticleRepository) FetchPublishedByColumnID(ctx context.Context, id int64) (model.Column, []model.Article, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	var column model.Column
	var columnName sql.NullString
	if err := r.db.QueryRowContext(ctx,
		"SELECT id, name FROM {{schema}}.`column` WHERE id = ? AND status = 1",
		id,
	).Scan(&column.ID, &columnName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Column{}, nil, fmt.Errorf("%w: id %d", ErrColumnNotFound, id)
		}
		return model.Column{}, nil, fmt.Errorf("resolve global column id %d: %w", id, err)
	}
	column.Name = columnName.String
	return r.fetchPublishedColumn(ctx, column)
}

func (r *ArticleRepository) fetchPublishedColumn(ctx context.Context, column model.Column) (model.Column, []model.Article, error) {
	articleRows, err := r.db.QueryContext(ctx, `
SELECT a.id, a.type, a.title, a.summary, a.content, a.cover,
       a.author, a.source, a.is_bold, a.default_color,
       a.url, a.publish_time
FROM {{schema}}.article_column_publish acp
INNER JOIN {{schema}}.article a ON a.id = acp.article_id
INNER JOIN {{schema}}.`+"`column`"+` c
        ON c.id = acp.column_id
       AND c.status = 1
       AND c.template_id = acp.template_id
WHERE acp.column_id = ?
  AND a.status = 1
  AND a.audit_status = 2
  AND a.type IN (1, 2, 3)
ORDER BY a.is_top DESC, a.publish_time DESC, a.id DESC`, column.ID)
	if err != nil {
		return model.Column{}, nil, fmt.Errorf("query global column %q: %w", column.Name, err)
	}
	defer articleRows.Close()

	articles := make([]model.Article, 0)
	for articleRows.Next() {
		article, err := scanDetailArticle(articleRows, column.ID)
		if err != nil {
			return model.Column{}, nil, fmt.Errorf("scan global column %q article: %w", column.Name, err)
		}
		articles = append(articles, article)
	}
	if err := articleRows.Err(); err != nil {
		return model.Column{}, nil, fmt.Errorf("iterate global column %q articles: %w", column.Name, err)
	}
	return column, articles, nil
}

func (r *ArticleRepository) FetchColumns(ctx context.Context) ([]model.Column, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, name FROM {{schema}}.`column` WHERE status = 1 ORDER BY id ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("query all columns: %w", err)
	}
	defer rows.Close()

	columns := make([]model.Column, 0)
	for rows.Next() {
		var (
			column model.Column
			name   sql.NullString
		)
		if err := rows.Scan(&column.ID, &name); err != nil {
			return nil, fmt.Errorf("scan global column: %w", err)
		}
		column.Name = name.String
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate global columns: %w", err)
	}
	return columns, nil
}

func (r *ArticleRepository) FetchArticleColumnMappingsBatch(ctx context.Context, afterID int64, limit int) ([]model.ArticleColumnMapping, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	if limit <= 0 {
		return nil, errors.New("mapping batch limit must be positive")
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, column_id, article_id
FROM {{schema}}.article_column_publish
WHERE id > ?
  AND EXISTS (
      SELECT 1 FROM {{schema}}.`+"`column`"+` c
      WHERE c.id = column_id
        AND c.status = 1
        AND c.template_id = template_id
  )
ORDER BY id ASC
LIMIT ?`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("query article-column mappings after %d: %w", afterID, err)
	}
	defer rows.Close()

	mappings := make([]model.ArticleColumnMapping, 0, limit)
	for rows.Next() {
		var mapping model.ArticleColumnMapping
		if err := rows.Scan(&mapping.ID, &mapping.ColumnID, &mapping.ArticleID); err != nil {
			return nil, fmt.Errorf("scan article-column mapping after %d: %w", afterID, err)
		}
		mappings = append(mappings, mapping)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate article-column mappings after %d: %w", afterID, err)
	}
	return mappings, nil
}

func (r *ArticleRepository) FetchListArticlesByIDs(ctx context.Context, ids []int64) ([]model.Article, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	if len(ids) == 0 {
		return nil, nil
	}
	query, args := articleIDsQuery(`
SELECT a.id, a.type, a.title, a.url, a.publish_time, a.is_top
FROM {{schema}}.article a
WHERE a.id IN (%s)
  AND a.status = 1
  AND a.audit_status = 2
  AND a.type IN (1, 2, 3)`, ids)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query list article batch: %w", err)
	}
	defer rows.Close()

	articles := make([]model.Article, 0, len(ids))
	for rows.Next() {
		article, err := scanBatchListArticle(rows)
		if err != nil {
			return nil, fmt.Errorf("scan list article batch: %w", err)
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate list article batch: %w", err)
	}
	return articles, nil
}

func (r *ArticleRepository) FetchDetailArticlesByIDs(ctx context.Context, ids []int64) ([]model.Article, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	if len(ids) == 0 {
		return nil, nil
	}
	query, args := articleIDsQuery(`
SELECT a.id, a.type, a.title, a.summary, a.content, a.cover,
       a.author, a.source, a.is_bold, a.default_color,
       a.url, a.publish_time, a.is_top
FROM {{schema}}.article a
WHERE a.id IN (%s)
  AND a.status = 1
  AND a.audit_status = 2
  AND a.type IN (1, 2)`, ids)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query detail article batch: %w", err)
	}
	defer rows.Close()

	articles := make([]model.Article, 0, len(ids))
	for rows.Next() {
		article, err := scanBatchDetailArticle(rows)
		if err != nil {
			return nil, fmt.Errorf("scan detail article batch: %w", err)
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate detail article batch: %w", err)
	}
	return articles, nil
}

// FetchArticleColumns returns every active column currently publishing a
// supported detail article. It is used to refresh all current list and main
// page references after an article is created or edited.
func (r *ArticleRepository) FetchArticleColumns(ctx context.Context, articleID int64) ([]model.Column, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `
SELECT DISTINCT c.id, c.name
FROM {{schema}}.article a
INNER JOIN {{schema}}.article_column_publish acp ON acp.article_id = a.id
INNER JOIN {{schema}}.`+"`column`"+` c ON c.id = acp.column_id
WHERE a.id = ?
  AND a.status = 1
  AND a.audit_status = 2
  AND a.type IN (1, 2)
	AND c.status = 1
	AND c.template_id = acp.template_id
ORDER BY c.id ASC`, articleID)
	if err != nil {
		return nil, fmt.Errorf("query article %d columns: %w", articleID, err)
	}
	defer rows.Close()
	columns := make([]model.Column, 0, 2)
	for rows.Next() {
		var column model.Column
		if err := rows.Scan(&column.ID, &column.Name); err != nil {
			return nil, fmt.Errorf("scan article %d column: %w", articleID, err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate article %d columns: %w", articleID, err)
	}
	if len(columns) == 0 {
		return nil, ErrArticleNotPublished
	}
	return columns, nil
}

// FetchArticleRelatedColumns resolves surviving publication relationships for
// an article even after the article itself has gone offline or been removed.
func (r *ArticleRepository) FetchArticleRelatedColumns(ctx context.Context, articleID int64) ([]model.Column, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `
SELECT DISTINCT c.id, c.name
FROM {{schema}}.article_column_publish acp
INNER JOIN {{schema}}.`+"`column`"+` c ON c.id = acp.column_id
WHERE acp.article_id = ?
	AND c.status = 1
	AND c.template_id = acp.template_id
ORDER BY c.id ASC`, articleID)
	if err != nil {
		return nil, fmt.Errorf("query article %d related columns: %w", articleID, err)
	}
	defer rows.Close()
	columns := make([]model.Column, 0, 2)
	for rows.Next() {
		var column model.Column
		var name sql.NullString
		if err := rows.Scan(&column.ID, &name); err != nil {
			return nil, fmt.Errorf("scan article %d related column: %w", articleID, err)
		}
		column.Name = name.String
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate article %d related columns: %w", articleID, err)
	}
	return columns, nil
}

func articleIDsQuery(format string, ids []int64) (string, []any) {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for index, id := range ids {
		placeholders[index] = "?"
		args[index] = id
	}
	return fmt.Sprintf(format, strings.Join(placeholders, ",")), args
}

func (r *ArticleRepository) FetchAllArticles(ctx context.Context) ([]model.Article, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `
SELECT selected.column_id,
       a.id, a.type, a.title, a.summary, a.content, a.cover,
       a.author, a.source, a.is_bold, a.default_color,
       a.url, a.publish_time
FROM (
    SELECT acp.article_id, MIN(acp.column_id) AS column_id
    FROM {{schema}}.article_column_publish acp
    INNER JOIN {{schema}}.`+"`column`"+` c
      ON c.id = acp.column_id
     AND c.status = 1
     AND c.template_id = acp.template_id
    GROUP BY acp.article_id
) selected
INNER JOIN {{schema}}.article a ON a.id = selected.article_id
WHERE a.status = 1
  AND a.audit_status = 2
	AND a.type IN (1, 2)
ORDER BY a.is_top DESC, a.publish_time DESC, a.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query all published articles: %w", err)
	}
	defer rows.Close()

	articles := make([]model.Article, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		var (
			article      model.Article
			title        sql.NullString
			summary      sql.NullString
			content      sql.NullString
			cover        sql.NullString
			author       sql.NullString
			source       sql.NullString
			bold         sql.NullInt64
			defaultColor sql.NullString
			articleURL   sql.NullString
			publishTime  sql.NullTime
		)
		if err := rows.Scan(
			&article.ColumnID,
			&article.ID, &article.Type, &title, &summary, &content, &cover,
			&author, &source, &bold, &defaultColor, &articleURL, &publishTime,
		); err != nil {
			return nil, fmt.Errorf("scan global published article: %w", err)
		}
		if _, exists := seen[article.ID]; exists {
			continue
		}
		seen[article.ID] = struct{}{}
		article.Title = title.String
		article.Summary = summary.String
		article.Content = content.String
		article.Cover = cover.String
		article.Author = author.String
		article.Source = source.String
		article.IsBold = bold.Valid && bold.Int64 != 0
		article.DefaultColor = defaultColor.String
		article.URL = articleURL.String
		if publishTime.Valid {
			article.PublishTime = publishTime.Time
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all published articles: %w", err)
	}
	return articles, nil
}

func (r *ArticleRepository) FetchByColumn(ctx context.Context, slot config.SlotConfig) ([]model.Article, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	columnID, err := r.resolveColumnByName(ctx, slot.Name)
	if err != nil {
		return nil, err
	}

	placeholders := make([]string, len(slot.Types))
	args := make([]any, 0, len(slot.Types)+2)
	args = append(args, columnID)
	for i, typ := range slot.Types {
		placeholders[i] = "?"
		args = append(args, typ)
	}
	args = append(args, slot.Limit)

	query := `
SELECT a.id, a.type, a.title, a.summary, a.content, a.cover,
       a.author, a.source, a.is_bold, a.default_color,
       a.url, a.publish_time
FROM (
    SELECT candidate.id
    FROM {{schema}}.article candidate
    INNER JOIN {{schema}}.article_column_publish acp
      ON acp.column_id = ? AND acp.article_id = candidate.id
    INNER JOIN {{schema}}.` + "`column`" + ` c
      ON c.id = acp.column_id
     AND c.status = 1
     AND c.template_id = acp.template_id
    WHERE candidate.status = 1
      AND candidate.audit_status = 2
      AND candidate.type IN (` + strings.Join(placeholders, ",") + `)
      AND candidate.type IN (1, 2, 3)
    ORDER BY candidate.is_top DESC, candidate.publish_time DESC, candidate.id DESC
    LIMIT ?
) selected
INNER JOIN {{schema}}.article a ON a.id = selected.id
ORDER BY a.is_top DESC, a.publish_time DESC, a.id DESC
`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query column %q: %w", slot.Name, err)
	}
	defer rows.Close()

	articles := make([]model.Article, 0, slot.Limit)
	for rows.Next() {
		var (
			article      model.Article
			articleTitle sql.NullString
			summary      sql.NullString
			content      sql.NullString
			cover        sql.NullString
			author       sql.NullString
			source       sql.NullString
			bold         sql.NullInt64
			defaultColor sql.NullString
			articleURL   sql.NullString
			publishTime  sql.NullTime
		)
		if err := rows.Scan(
			&article.ID, &article.Type, &articleTitle, &summary, &content, &cover,
			&author, &source, &bold, &defaultColor, &articleURL, &publishTime,
		); err != nil {
			return nil, fmt.Errorf("scan column %q: %w", slot.Name, err)
		}
		article.Title = articleTitle.String
		article.ColumnID = columnID
		article.Summary = summary.String
		article.Content = content.String
		article.Cover = cover.String
		article.Author = author.String
		article.Source = source.String
		article.IsBold = bold.Valid && bold.Int64 != 0
		article.DefaultColor = defaultColor.String
		article.URL = articleURL.String
		if publishTime.Valid {
			article.PublishTime = publishTime.Time
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate column %q: %w", slot.Name, err)
	}
	return articles, nil
}

func (r *ArticleRepository) FetchMonthlyStatistics(ctx context.Context, title, throughMonth string) ([]model.Article, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	throughMonth = strings.TrimSpace(throughMonth)
	query := `
SELECT a.id, a.type, a.title, a.summary, a.content, a.cover,
       a.author, a.source, a.is_bold, a.default_color,
       a.url, a.publish_time
FROM {{schema}}.article a
WHERE a.status = 1
  AND a.audit_status = 2
  AND a.type = 3
  AND a.title = ?`
	args := []any{strings.TrimSpace(title)}
	if throughMonth != "" {
		period, err := time.Parse("2006-01", throughMonth)
		if err != nil {
			return nil, fmt.Errorf("invalid monthly statistics cutoff %q: %w", throughMonth, err)
		}
		query += `
  AND (
      a.summary LIKE ?
      OR (
          a.summary LIKE ?
          AND a.summary <= ?
      )
  )`
		args = append(args, fmt.Sprintf("%d-%%", period.Year()-1), fmt.Sprintf("%d-%%", period.Year()), throughMonth)
	}
	query += "\nORDER BY a.summary DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query monthly statistics %q through %s: %w", title, throughMonth, err)
	}
	defer rows.Close()

	articles := make([]model.Article, 0, 24)
	for rows.Next() {
		var (
			article      model.Article
			articleTitle sql.NullString
			summary      sql.NullString
			content      sql.NullString
			cover        sql.NullString
			author       sql.NullString
			source       sql.NullString
			bold         sql.NullInt64
			defaultColor sql.NullString
			articleURL   sql.NullString
			publishTime  sql.NullTime
		)
		if err := rows.Scan(
			&article.ID, &article.Type, &articleTitle, &summary, &content, &cover,
			&author, &source, &bold, &defaultColor, &articleURL, &publishTime,
		); err != nil {
			return nil, fmt.Errorf("scan monthly statistics %q: %w", title, err)
		}
		article.Title = articleTitle.String
		article.Summary = summary.String
		article.Content = content.String
		article.Cover = cover.String
		article.Author = author.String
		article.Source = source.String
		article.IsBold = bold.Valid && bold.Int64 != 0
		article.DefaultColor = defaultColor.String
		article.URL = articleURL.String
		if publishTime.Valid {
			article.PublishTime = publishTime.Time
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monthly statistics %q: %w", title, err)
	}
	return articles, nil
}

func (r *ArticleRepository) FetchStatisticsTitles(ctx context.Context, columnName string) ([]string, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `
SELECT acp.article_title
FROM {{schema}}.article_column_publish acp
INNER JOIN {{schema}}.`+"`column`"+` c
        ON c.id = acp.column_id
       AND c.status = 1
       AND c.template_id = acp.template_id
WHERE c.name = ?
GROUP BY article_title`, strings.TrimSpace(columnName))
	if err != nil {
		return nil, fmt.Errorf("query statistics titles from column %q: %w", columnName, err)
	}
	defer rows.Close()

	titles := make([]string, 0)
	for rows.Next() {
		var title sql.NullString
		if err := rows.Scan(&title); err != nil {
			return nil, fmt.Errorf("scan statistics title from column %q: %w", columnName, err)
		}
		value := strings.TrimSpace(title.String)
		if value != "" {
			titles = append(titles, value)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate statistics titles from column %q: %w", columnName, err)
	}
	return titles, nil
}

func (r *ArticleRepository) resolveColumnByName(ctx context.Context, name string) (int64, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id FROM {{schema}}.`column` WHERE template_id = ? AND name = ? AND status = 1 ORDER BY id ASC LIMIT 2",
		r.templateID, name,
	)
	if err != nil {
		return 0, fmt.Errorf("查询栏目“%s”失败：%w", name, err)
	}
	defer rows.Close()
	ids := make([]int64, 0, 2)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("读取栏目“%s”失败：%w", name, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("遍历栏目“%s”失败：%w", name, err)
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("%w：“%s”", ErrColumnNotFound, name)
	}
	if len(ids) > 1 {
		return 0, fmt.Errorf("%w：“%s”，匹配到 %d 条记录", ErrColumnNotUnique, name, len(ids))
	}
	return ids[0], nil
}

func (r *ArticleRepository) fetchColumn(ctx context.Context, columnID int64) (model.Column, error) {
	var column model.Column
	err := r.db.QueryRowContext(ctx,
		"SELECT id, name FROM {{schema}}.`column` WHERE id = ? AND status = 1",
		columnID,
	).Scan(&column.ID, &column.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Column{}, ErrColumnNotFound
	}
	if err != nil {
		return model.Column{}, fmt.Errorf("query column %d: %w", columnID, err)
	}
	return column, nil
}

func (r *ArticleRepository) FetchColumnArticles(ctx context.Context, columnID int64) (model.Column, []model.Article, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	column, err := r.fetchColumn(ctx, columnID)
	if err != nil {
		return model.Column{}, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT a.id, a.type, a.title, a.url, a.publish_time
FROM {{schema}}.article_column_publish acp
INNER JOIN {{schema}}.article a ON a.id = acp.article_id
INNER JOIN {{schema}}.`+"`column`"+` c
        ON c.id = acp.column_id
       AND c.status = 1
       AND c.template_id = acp.template_id
WHERE acp.column_id = ?
  AND a.status = 1
  AND a.audit_status = 2
  AND a.type IN (1, 2, 3)
ORDER BY a.is_top DESC, a.publish_time DESC, a.id DESC`, columnID)
	if err != nil {
		return model.Column{}, nil, fmt.Errorf("query column %d articles: %w", columnID, err)
	}
	defer rows.Close()
	articles := make([]model.Article, 0)
	for rows.Next() {
		article, err := scanListArticle(rows, columnID)
		if err != nil {
			return model.Column{}, nil, fmt.Errorf("scan column %d article: %w", columnID, err)
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return model.Column{}, nil, fmt.Errorf("iterate column %d articles: %w", columnID, err)
	}
	return column, articles, nil
}

func (r *ArticleRepository) FetchArticle(ctx context.Context, articleID int64) (model.Column, model.Article, error) {
	ctx, cancel := withDatabaseOperationTimeout(ctx)
	defer cancel()
	row := r.db.QueryRowContext(ctx, `
SELECT acp.column_id,
       a.id, a.type, a.title, a.summary, a.content, a.cover,
       a.author, a.source, a.is_bold, a.default_color,
       a.url, a.publish_time
FROM {{schema}}.article a
INNER JOIN {{schema}}.article_column_publish acp
        ON acp.article_id = a.id
INNER JOIN {{schema}}.`+"`column`"+` c
       ON c.id = acp.column_id
       AND c.status = 1
       AND c.template_id = acp.template_id
WHERE a.id = ?
  AND a.status = 1
  AND a.audit_status = 2
  AND a.type IN (1, 2)
ORDER BY acp.column_id ASC
LIMIT 1`, articleID)
	article, err := scanNamedColumnDetailArticle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Column{}, model.Article{}, ErrArticleNotPublished
	}
	if err != nil {
		return model.Column{}, model.Article{}, fmt.Errorf("query article %d: %w", articleID, err)
	}
	column, err := r.fetchColumn(ctx, article.ColumnID)
	if err != nil {
		return model.Column{}, model.Article{}, err
	}
	return column, article, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanListArticle(row rowScanner, columnID int64) (model.Article, error) {
	var (
		article     model.Article
		title       sql.NullString
		articleURL  sql.NullString
		publishTime sql.NullTime
	)
	err := row.Scan(&article.ID, &article.Type, &title, &articleURL, &publishTime)
	if err != nil {
		return model.Article{}, err
	}
	article.ColumnID = columnID
	article.Title = title.String
	article.URL = articleURL.String
	if publishTime.Valid {
		article.PublishTime = publishTime.Time
	}
	return article, nil
}

func scanBatchListArticle(row rowScanner) (model.Article, error) {
	var (
		article     model.Article
		title       sql.NullString
		articleURL  sql.NullString
		publishTime sql.NullTime
		isTop       sql.NullInt64
	)
	if err := row.Scan(&article.ID, &article.Type, &title, &articleURL, &publishTime, &isTop); err != nil {
		return model.Article{}, err
	}
	article.Title = title.String
	article.URL = articleURL.String
	article.IsTop = isTop.Valid && isTop.Int64 != 0
	if publishTime.Valid {
		article.PublishTime = publishTime.Time
	}
	return article, nil
}

func scanBatchDetailArticle(row rowScanner) (model.Article, error) {
	var (
		article      model.Article
		title        sql.NullString
		summary      sql.NullString
		content      sql.NullString
		cover        sql.NullString
		author       sql.NullString
		source       sql.NullString
		bold         sql.NullInt64
		defaultColor sql.NullString
		articleURL   sql.NullString
		publishTime  sql.NullTime
		isTop        sql.NullInt64
	)
	if err := row.Scan(
		&article.ID, &article.Type, &title, &summary, &content, &cover,
		&author, &source, &bold, &defaultColor, &articleURL, &publishTime, &isTop,
	); err != nil {
		return model.Article{}, err
	}
	article.Title = title.String
	article.Summary = summary.String
	article.Content = content.String
	article.Cover = cover.String
	article.Author = author.String
	article.Source = source.String
	article.IsBold = bold.Valid && bold.Int64 != 0
	article.IsTop = isTop.Valid && isTop.Int64 != 0
	article.DefaultColor = defaultColor.String
	article.URL = articleURL.String
	if publishTime.Valid {
		article.PublishTime = publishTime.Time
	}
	return article, nil
}

func scanDetailArticle(row rowScanner, columnID int64) (model.Article, error) {
	var (
		article      model.Article
		title        sql.NullString
		summary      sql.NullString
		content      sql.NullString
		cover        sql.NullString
		author       sql.NullString
		source       sql.NullString
		bold         sql.NullInt64
		defaultColor sql.NullString
		articleURL   sql.NullString
		publishTime  sql.NullTime
	)
	err := row.Scan(
		&article.ID, &article.Type, &title, &summary, &content, &cover,
		&author, &source, &bold, &defaultColor, &articleURL, &publishTime,
	)
	if err != nil {
		return model.Article{}, err
	}
	article.ColumnID = columnID
	article.Title = title.String
	article.Summary = summary.String
	article.Content = content.String
	article.Cover = cover.String
	article.Author = author.String
	article.Source = source.String
	article.IsBold = bold.Valid && bold.Int64 != 0
	article.DefaultColor = defaultColor.String
	article.URL = articleURL.String
	if publishTime.Valid {
		article.PublishTime = publishTime.Time
	}
	return article, nil
}

func scanNamedColumnDetailArticle(row rowScanner) (model.Article, error) {
	var (
		columnID     int64
		article      model.Article
		title        sql.NullString
		summary      sql.NullString
		content      sql.NullString
		cover        sql.NullString
		author       sql.NullString
		source       sql.NullString
		bold         sql.NullInt64
		defaultColor sql.NullString
		articleURL   sql.NullString
		publishTime  sql.NullTime
	)
	err := row.Scan(
		&columnID, &article.ID, &article.Type, &title, &summary, &content, &cover,
		&author, &source, &bold, &defaultColor, &articleURL, &publishTime,
	)
	if err != nil {
		return model.Article{}, err
	}
	article.ColumnID = columnID
	article.Title = title.String
	article.Summary = summary.String
	article.Content = content.String
	article.Cover = cover.String
	article.Author = author.String
	article.Source = source.String
	article.IsBold = bold.Valid && bold.Int64 != 0
	article.DefaultColor = defaultColor.String
	article.URL = articleURL.String
	if publishTime.Valid {
		article.PublishTime = publishTime.Time
	}
	return article, nil
}
