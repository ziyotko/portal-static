package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"miic-portal/backend/internal/model"
)

var (
	ErrPageNotFound        = errors.New("page not found")
	ErrPageNotUnique       = errors.New("page name is not unique")
	ErrColumnNotFound      = errors.New("column not found")
	ErrColumnNotUnique     = errors.New("column name is not unique")
	ErrArticleNotPublished = errors.New("article not found or not published")
)

type Repository struct {
	db     *sql.DB
	pageID int64
}

func ResolvePageID(ctx context.Context, db *sql.DB, name string) (int64, error) {
	rows, err := db.QueryContext(ctx, "SELECT id FROM page WHERE name=? AND status=1 AND deleted_at IS NULL ORDER BY id LIMIT 2", strings.TrimSpace(name))
	if err != nil {
		return 0, fmt.Errorf("resolve page: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return 0, ErrPageNotFound
	}
	if len(ids) > 1 {
		return 0, ErrPageNotUnique
	}
	return ids[0], rows.Err()
}

func New(db *sql.DB, pageID int64) *Repository { return &Repository{db: db, pageID: pageID} }

func (r *Repository) FetchByColumnName(ctx context.Context, name string, limit int) (model.Column, []model.Article, error) {
	column, err := r.resolvePageColumn(ctx, r.pageID, name)
	if err != nil {
		return model.Column{}, nil, err
	}
	items, err := r.fetchColumnArticles(ctx, column.ID, limit)
	return column, items, err
}

func (r *Repository) FetchPageColumnArticles(ctx context.Context, pageName, columnName string, limit int) (model.Column, []model.Article, error) {
	pageID, err := ResolvePageID(ctx, r.db, pageName)
	if err != nil {
		return model.Column{}, nil, err
	}
	column, err := r.resolvePageColumn(ctx, pageID, columnName)
	if err != nil {
		return model.Column{}, nil, err
	}
	items, err := r.fetchColumnArticles(ctx, column.ID, limit)
	return column, items, err
}

func (r *Repository) FetchPageColumns(ctx context.Context, pageName string, parentID int64) ([]model.Column, error) {
	pageID, err := ResolvePageID(ctx, r.db, pageName)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,code,page_id,parent_id,COALESCE(description,''),sort
FROM `+"`column`"+` WHERE page_id=? AND parent_id=? AND status=1 AND deleted_at IS NULL ORDER BY sort,id`, pageID, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.Column
	seenNames := map[string]bool{}
	seenCodes := map[string]bool{}
	for rows.Next() {
		var column model.Column
		if err := rows.Scan(&column.ID, &column.Name, &column.Code, &column.PageID, &column.ParentID, &column.Description, &column.Sort); err != nil {
			return nil, err
		}
		nameKey := strings.TrimSpace(column.Name)
		codeKey := strings.TrimSpace(column.Code)
		if seenNames[nameKey] || (codeKey != "" && seenCodes[codeKey]) {
			return nil, fmt.Errorf("%w: page=%s parent=%d name=%s code=%s", ErrColumnNotUnique, pageName, parentID, column.Name, column.Code)
		}
		seenNames[nameKey] = true
		if codeKey != "" {
			seenCodes[codeKey] = true
		}
		result = append(result, column)
	}
	return result, rows.Err()
}

func (r *Repository) ResolveGlobalColumnID(ctx context.Context, name string) (int64, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id FROM `column` WHERE name=? AND status=1 AND deleted_at IS NULL ORDER BY id LIMIT 2", strings.TrimSpace(name))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return 0, ErrColumnNotFound
	}
	if len(ids) > 1 {
		return 0, ErrColumnNotUnique
	}
	return ids[0], rows.Err()
}

func (r *Repository) FetchColumns(ctx context.Context) ([]model.Column, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id,c.name,c.code,c.page_id,p.name,c.parent_id,COALESCE(c.description,''),c.sort
FROM `+"`column`"+` c INNER JOIN page p ON p.id=c.page_id AND p.status=1 AND p.deleted_at IS NULL
WHERE c.status=1 AND c.deleted_at IS NULL ORDER BY c.page_id,c.parent_id,c.sort,c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.Column
	for rows.Next() {
		var c model.Column
		if err := rows.Scan(&c.ID, &c.Name, &c.Code, &c.PageID, &c.PageName, &c.ParentID, &c.Description, &c.Sort); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *Repository) FetchColumnArticles(ctx context.Context, id int64) (model.Column, []model.Article, error) {
	column, err := r.resolveColumnByID(ctx, id)
	if err != nil {
		return model.Column{}, nil, err
	}
	items, err := r.fetchColumnArticles(ctx, id, 0)
	return column, items, err
}

func (r *Repository) fetchColumnArticles(ctx context.Context, id int64, limit int) ([]model.Article, error) {
	query := `SELECT a.id,a.type,a.title,COALESCE(a.summary,''),COALESCE(a.content,''),COALESCE(a.cover,''),
COALESCE(NULLIF(acp.author,''),a.author,''),COALESCE(NULLIF(acp.source,''),a.source,''),
COALESCE(acp.is_bold,0),COALESCE(acp.is_top,0),COALESCE(NULLIF(acp.color,''),a.default_color,''),
COALESCE(a.url,''),a.publish_time
FROM article_column_publish acp INNER JOIN article a ON a.id=acp.article_id
INNER JOIN ` + "`column`" + ` c ON c.id=acp.column_id AND c.status=1 AND c.deleted_at IS NULL
INNER JOIN page p ON p.id=c.page_id AND p.status=1 AND p.deleted_at IS NULL
WHERE acp.column_id=? AND acp.deleted_at IS NULL AND a.status=1 AND a.audit_status=2 AND a.deleted_at IS NULL
AND a.type IN (1,2) ORDER BY acp.is_top DESC,a.publish_time DESC,a.id DESC`
	args := []any{id}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	items, err := r.queryArticles(ctx, query, args...)
	for i := range items {
		items[i].ColumnID = id
	}
	return items, err
}

func (r *Repository) FetchArticle(ctx context.Context, id int64) (model.Column, model.Article, error) {
	query := `SELECT c.id,c.name,c.code,c.page_id,c.parent_id,COALESCE(c.description,''),c.sort,
a.id,a.type,a.title,COALESCE(a.summary,''),COALESCE(a.content,''),COALESCE(a.cover,''),
COALESCE(NULLIF(acp.author,''),a.author,''),COALESCE(NULLIF(acp.source,''),a.source,''),
COALESCE(acp.is_bold,0),COALESCE(acp.is_top,0),COALESCE(NULLIF(acp.color,''),a.default_color,''),
COALESCE(a.url,''),a.publish_time
FROM article a INNER JOIN article_column_publish acp ON acp.article_id=a.id
INNER JOIN ` + "`column`" + ` c ON c.id=acp.column_id AND c.status=1 AND c.deleted_at IS NULL
INNER JOIN page p ON p.id=c.page_id AND p.status=1 AND p.deleted_at IS NULL
WHERE a.id=? AND acp.deleted_at IS NULL AND a.status=1 AND a.audit_status=2 AND a.deleted_at IS NULL AND a.type IN (1,2)
ORDER BY acp.is_top DESC,c.sort,c.id LIMIT 1`
	var c model.Column
	var a model.Article
	err := r.db.QueryRowContext(ctx, query, id).Scan(&c.ID, &c.Name, &c.Code, &c.PageID, &c.ParentID, &c.Description, &c.Sort,
		&a.ID, &a.Type, &a.Title, &a.Summary, &a.Content, &a.Cover, &a.Author, &a.Source, &a.IsBold, &a.IsTop, &a.DefaultColor, &a.URL, &a.PublishTime)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Column{}, model.Article{}, ErrArticleNotPublished
	}
	if err != nil {
		return model.Column{}, model.Article{}, err
	}
	a.ColumnID = c.ID
	return c, a, nil
}

// FetchArticleColumns returns every active column currently publishing a
// supported detail article. Related static refreshes use it to cover all of an
// article's current list and main-page references.
func (r *Repository) FetchArticleColumns(ctx context.Context, articleID int64) ([]model.Column, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT c.id,c.name,c.code,c.page_id,p.name,c.parent_id,COALESCE(c.description,''),c.sort
FROM article a INNER JOIN article_column_publish acp ON acp.article_id=a.id AND acp.deleted_at IS NULL
INNER JOIN `+"`column`"+` c ON c.id=acp.column_id AND c.status=1 AND c.deleted_at IS NULL
INNER JOIN page p ON p.id=c.page_id AND p.status=1 AND p.deleted_at IS NULL
WHERE a.id=? AND a.status=1 AND a.audit_status=2 AND a.deleted_at IS NULL AND a.type IN (1,2)
ORDER BY c.id`, articleID)
	if err != nil {
		return nil, fmt.Errorf("query article %d columns: %w", articleID, err)
	}
	defer rows.Close()
	columns := make([]model.Column, 0, 2)
	for rows.Next() {
		var column model.Column
		if err := rows.Scan(&column.ID, &column.Name, &column.Code, &column.PageID, &column.PageName, &column.ParentID, &column.Description, &column.Sort); err != nil {
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

// FetchArticleRelatedColumns includes surviving and soft-deleted publication
// relationships even when the article is offline, deleted or no longer exists.
// Only columns and pages that can still be rendered are returned.
func (r *Repository) FetchArticleRelatedColumns(ctx context.Context, articleID int64) ([]model.Column, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT c.id,c.name,c.code,c.page_id,p.name,c.parent_id,COALESCE(c.description,''),c.sort
FROM article_column_publish acp
INNER JOIN `+"`column`"+` c ON c.id=acp.column_id AND c.status=1 AND c.deleted_at IS NULL
INNER JOIN page p ON p.id=c.page_id AND p.status=1 AND p.deleted_at IS NULL
WHERE acp.article_id=? ORDER BY c.id`, articleID)
	if err != nil {
		return nil, fmt.Errorf("query article %d related columns: %w", articleID, err)
	}
	defer rows.Close()
	var columns []model.Column
	for rows.Next() {
		var column model.Column
		if err := rows.Scan(&column.ID, &column.Name, &column.Code, &column.PageID, &column.PageName, &column.ParentID, &column.Description, &column.Sort); err != nil {
			return nil, fmt.Errorf("scan related column: %w", err)
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func (r *Repository) FetchAllArticles(ctx context.Context) ([]model.Article, error) {
	query := `SELECT a.id,a.type,a.title,COALESCE(a.summary,''),COALESCE(a.content,''),COALESCE(a.cover,''),
COALESCE(a.author,''),COALESCE(a.source,''),COALESCE(a.is_bold,0),COALESCE(a.is_top,0),
COALESCE(a.default_color,''),COALESCE(a.url,''),a.publish_time
FROM article a WHERE a.status=1 AND a.audit_status=2 AND a.deleted_at IS NULL AND a.type IN (1,2)
AND EXISTS(SELECT 1 FROM article_column_publish acp INNER JOIN ` + "`column`" + ` c ON c.id=acp.column_id AND c.status=1 AND c.deleted_at IS NULL INNER JOIN page p ON p.id=c.page_id AND p.status=1 AND p.deleted_at IS NULL WHERE acp.article_id=a.id AND acp.deleted_at IS NULL)
ORDER BY a.id`
	return r.queryArticles(ctx, query)
}

func (r *Repository) FetchAttachments(ctx context.Context, articleID int64) ([]model.Attachment, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id,article_id,name,url,size FROM article_attachment WHERE article_id=? ORDER BY id", articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.Attachment
	for rows.Next() {
		var attachment model.Attachment
		if err := rows.Scan(&attachment.ID, &attachment.ArticleID, &attachment.Name, &attachment.URL, &attachment.Size); err != nil {
			return nil, err
		}
		result = append(result, attachment)
	}
	return result, rows.Err()
}

func (r *Repository) resolvePageColumn(ctx context.Context, pageID int64, name string) (model.Column, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,code,page_id,parent_id,COALESCE(description,''),sort FROM `+"`column`"+`
WHERE page_id=? AND name=? AND status=1 AND deleted_at IS NULL ORDER BY id LIMIT 2`, pageID, strings.TrimSpace(name))
	if err != nil {
		return model.Column{}, err
	}
	defer rows.Close()
	var result []model.Column
	for rows.Next() {
		var c model.Column
		if err := rows.Scan(&c.ID, &c.Name, &c.Code, &c.PageID, &c.ParentID, &c.Description, &c.Sort); err != nil {
			return model.Column{}, err
		}
		result = append(result, c)
	}
	if len(result) == 0 {
		return model.Column{}, ErrColumnNotFound
	}
	if len(result) > 1 {
		return model.Column{}, ErrColumnNotUnique
	}
	return result[0], rows.Err()
}

func (r *Repository) resolveColumnByID(ctx context.Context, id int64) (model.Column, error) {
	var c model.Column
	err := r.db.QueryRowContext(ctx, `SELECT c.id,c.name,c.code,c.page_id,c.parent_id,COALESCE(c.description,''),c.sort
FROM `+"`column`"+` c INNER JOIN page p ON p.id=c.page_id AND p.status=1 AND p.deleted_at IS NULL
WHERE c.id=? AND c.status=1 AND c.deleted_at IS NULL`, id).Scan(&c.ID, &c.Name, &c.Code, &c.PageID, &c.ParentID, &c.Description, &c.Sort)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Column{}, ErrColumnNotFound
	}
	return c, err
}

func (r *Repository) queryArticles(ctx context.Context, query string, args ...any) ([]model.Article, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.Article
	for rows.Next() {
		var a model.Article
		if err := rows.Scan(&a.ID, &a.Type, &a.Title, &a.Summary, &a.Content, &a.Cover, &a.Author, &a.Source, &a.IsBold, &a.IsTop, &a.DefaultColor, &a.URL, &a.PublishTime); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
