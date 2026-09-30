package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"portal-static/internal/adapters/camie/model"
	"portal-static/internal/contracts"
	"portal-static/internal/sources/portalcms"
)

var (
	ErrTemplateNotFound    = contracts.ErrTemplateNotFound
	ErrTemplateNotUnique   = contracts.ErrTemplateNotUnique
	ErrColumnNotFound      = contracts.ErrColumnNotFound
	ErrColumnNotUnique     = contracts.ErrColumnNotUnique
	ErrArticleNotPublished = contracts.ErrArticleNotPublished
)

type Repository struct {
	db         *portalcms.Store
	templateID int64
}

func New(db *sql.DB, templateID int64) *Repository {
	store, err := portalcms.NewStore(db, "", false)
	if err != nil {
		panic(err)
	}
	return NewWithStore(store, templateID)
}

func NewWithStore(store *portalcms.Store, templateID int64) *Repository {
	return &Repository{db: store, templateID: templateID}
}

func (r *Repository) FetchPageColumns(ctx context.Context, _ string, parentID int64) ([]model.Column, error) {
	// CAMIE navigation spans several CMS page templates. Stable column.code is
	// the site-wide key, so the adapter reads the complete enabled hierarchy
	// while still requiring every column to belong to an enabled template.
	rows, err := r.db.QueryContext(ctx, `SELECT c.id,c.name,c.code,c.template_id,c.parent_id,COALESCE(c.description,''),c.sort
FROM {{schema}}.`+"`column`"+` c
INNER JOIN {{schema}}.template t ON t.id=c.template_id AND t.status=1
WHERE c.parent_id=? AND c.status=1 ORDER BY c.sort,c.id`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.Column
	seenNames := map[string]bool{}
	seenCodes := map[string]bool{}
	for rows.Next() {
		var column model.Column
		if err := rows.Scan(&column.ID, &column.Name, &column.Code, &column.TemplateID, &column.ParentID, &column.Description, &column.Sort); err != nil {
			return nil, err
		}
		nameKey := strings.TrimSpace(column.Name)
		codeKey := strings.TrimSpace(column.Code)
		if seenNames[nameKey] || (codeKey != "" && seenCodes[codeKey]) {
			return nil, fmt.Errorf("%w: parent=%d name=%s code=%s", ErrColumnNotUnique, parentID, column.Name, column.Code)
		}
		seenNames[nameKey] = true
		if codeKey != "" {
			seenCodes[codeKey] = true
		}
		result = append(result, column)
	}
	return result, rows.Err()
}

func (r *Repository) FetchColumns(ctx context.Context) ([]model.Column, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id,c.name,c.code,c.template_id,t.name,c.parent_id,COALESCE(c.description,''),c.sort
FROM {{schema}}.`+"`column`"+` c INNER JOIN {{schema}}.template t ON t.id=c.template_id AND t.status=1
WHERE c.status=1 ORDER BY c.template_id,c.parent_id,c.sort,c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.Column
	for rows.Next() {
		var c model.Column
		if err := rows.Scan(&c.ID, &c.Name, &c.Code, &c.TemplateID, &c.TemplateName, &c.ParentID, &c.Description, &c.Sort); err != nil {
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
FROM {{schema}}.article_column_publish acp INNER JOIN {{schema}}.article a ON a.id=acp.article_id
INNER JOIN {{schema}}.` + "`column`" + ` c ON c.id=acp.column_id AND c.status=1 AND c.template_id=acp.template_id
INNER JOIN {{schema}}.template t ON t.id=c.template_id AND t.status=1
WHERE acp.column_id=? AND a.status=1 AND a.audit_status=2 AND a.publish_time<=NOW()
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
	query := `SELECT c.id,c.name,c.code,c.template_id,c.parent_id,COALESCE(c.description,''),c.sort,
a.id,a.type,a.title,COALESCE(a.summary,''),COALESCE(a.content,''),COALESCE(a.cover,''),
COALESCE(NULLIF(acp.author,''),a.author,''),COALESCE(NULLIF(acp.source,''),a.source,''),
COALESCE(acp.is_bold,0),COALESCE(acp.is_top,0),COALESCE(NULLIF(acp.color,''),a.default_color,''),
COALESCE(a.url,''),a.publish_time
FROM {{schema}}.article a INNER JOIN {{schema}}.article_column_publish acp ON acp.article_id=a.id
INNER JOIN {{schema}}.` + "`column`" + ` c ON c.id=acp.column_id AND c.status=1 AND c.template_id=acp.template_id
INNER JOIN {{schema}}.template t ON t.id=c.template_id AND t.status=1
WHERE a.id=? AND a.status=1 AND a.audit_status=2 AND a.publish_time<=NOW() AND a.type IN (1,2)
ORDER BY acp.is_top DESC,c.sort,c.id LIMIT 1`
	var c model.Column
	var a model.Article
	err := r.db.QueryRowContext(ctx, query, id).Scan(&c.ID, &c.Name, &c.Code, &c.TemplateID, &c.ParentID, &c.Description, &c.Sort,
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
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT c.id,c.name,c.code,c.template_id,t.name,c.parent_id,COALESCE(c.description,''),c.sort
FROM {{schema}}.article a INNER JOIN {{schema}}.article_column_publish acp ON acp.article_id=a.id
INNER JOIN {{schema}}.`+"`column`"+` c ON c.id=acp.column_id AND c.status=1 AND c.template_id=acp.template_id
INNER JOIN {{schema}}.template t ON t.id=c.template_id AND t.status=1
WHERE a.id=? AND a.status=1 AND a.audit_status=2 AND a.publish_time<=NOW() AND a.type IN (1,2)
ORDER BY c.id`, articleID)
	if err != nil {
		return nil, fmt.Errorf("query article %d columns: %w", articleID, err)
	}
	defer rows.Close()
	columns := make([]model.Column, 0, 2)
	for rows.Next() {
		var column model.Column
		if err := rows.Scan(&column.ID, &column.Name, &column.Code, &column.TemplateID, &column.TemplateName, &column.ParentID, &column.Description, &column.Sort); err != nil {
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

// FetchArticleRelatedColumns includes surviving publication relationships even
// when the article is offline or has been removed. Physical deletion means
// removed relationships cannot be reconstructed after the fact.
func (r *Repository) FetchArticleRelatedColumns(ctx context.Context, articleID int64) ([]model.Column, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT c.id,c.name,c.code,c.template_id,t.name,c.parent_id,COALESCE(c.description,''),c.sort
FROM {{schema}}.article_column_publish acp
INNER JOIN {{schema}}.`+"`column`"+` c ON c.id=acp.column_id AND c.status=1 AND c.template_id=acp.template_id
INNER JOIN {{schema}}.template t ON t.id=c.template_id AND t.status=1
WHERE acp.article_id=? ORDER BY c.id`, articleID)
	if err != nil {
		return nil, fmt.Errorf("query article %d related columns: %w", articleID, err)
	}
	defer rows.Close()
	var columns []model.Column
	for rows.Next() {
		var column model.Column
		if err := rows.Scan(&column.ID, &column.Name, &column.Code, &column.TemplateID, &column.TemplateName, &column.ParentID, &column.Description, &column.Sort); err != nil {
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
FROM {{schema}}.article a WHERE a.status=1 AND a.audit_status=2 AND a.publish_time<=NOW() AND a.type IN (1,2)
AND EXISTS(SELECT 1 FROM {{schema}}.article_column_publish acp INNER JOIN {{schema}}.` + "`column`" + ` c ON c.id=acp.column_id AND c.status=1 AND c.template_id=acp.template_id INNER JOIN {{schema}}.template t ON t.id=c.template_id AND t.status=1 WHERE acp.article_id=a.id)
ORDER BY a.id`
	return r.queryArticles(ctx, query)
}

func (r *Repository) FetchAttachments(ctx context.Context, articleID int64) ([]model.Attachment, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id,article_id,name,url,size FROM {{schema}}.article_attachment WHERE article_id=? ORDER BY id", articleID)
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

func (r *Repository) resolveColumnByID(ctx context.Context, id int64) (model.Column, error) {
	var c model.Column
	err := r.db.QueryRowContext(ctx, `SELECT c.id,c.name,c.code,c.template_id,c.parent_id,COALESCE(c.description,''),c.sort
FROM {{schema}}.`+"`column`"+` c INNER JOIN {{schema}}.template t ON t.id=c.template_id AND t.status=1
WHERE c.id=? AND c.status=1`, id).Scan(&c.ID, &c.Name, &c.Code, &c.TemplateID, &c.ParentID, &c.Description, &c.Sort)
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
