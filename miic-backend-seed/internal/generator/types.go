package generator

import (
	"context"
	"html/template"
	"time"

	"miic-portal/backend/internal/model"
)

type Source interface {
	FetchByColumnName(context.Context, string, int) (model.Column, []model.Article, error)
	FetchPageColumns(context.Context, string, int64) ([]model.Column, error)
	FetchPageColumnArticles(context.Context, string, string, int) (model.Column, []model.Article, error)
	FetchAttachments(context.Context, int64) ([]model.Attachment, error)
	ResolveGlobalColumnID(context.Context, string) (int64, error)
	FetchColumns(context.Context) ([]model.Column, error)
	FetchColumnArticles(context.Context, int64) (model.Column, []model.Article, error)
	FetchArticle(context.Context, int64) (model.Column, model.Article, error)
	FetchArticleColumns(context.Context, int64) ([]model.Column, error)
	FetchArticleRelatedColumns(context.Context, int64) ([]model.Column, error)
	FetchAllArticles(context.Context) ([]model.Article, error)
}

type ArticleView struct {
	ID          int64
	Category    string
	CategoryKey string
	Title       string
	Summary     string
	Content     template.HTML
	Cover       string
	Href        string
	Author      string
	Source      string
	DateISO     string
	DateDot     string
	DateCN      string
	External    bool
	Bold        bool
	Color       string
	Feature     bool
}

type CategoryView struct{ Key, Title string }

type NewsPageData struct {
	GeneratedAt string
	Items       []ArticleView
	Categories  []CategoryView
}

type ListPageData struct {
	GeneratedAt string
	RootPrefix  string
	Column      model.Column
	Items       []ArticleView
	Page        int
	TotalPages  int
	Previous    string
	Next        string
	Pages       []int
}

type ArticlePageData struct {
	GeneratedAt string
	RootPrefix  string
	ActiveRoot  string
	Column      model.Column
	Article     ArticleView
	BackHref    string
	BackLabel   string
}

type GenerationResult struct {
	GeneratedAt       time.Time `json:"generated_at"`
	DurationSeconds   float64   `json:"duration_seconds"`
	GeneratedFiles    int       `json:"generated_files"`
	GeneratedDetails  int       `json:"generated_details"`
	GeneratedLists    int       `json:"generated_lists"`
	Output            string    `json:"output"`
	GeneratedPages    int       `json:"generated_pages,omitempty"`
	GeneratedColumns  int       `json:"generated_columns,omitempty"`
	GeneratedArticles int       `json:"generated_articles,omitempty"`
	TotalItems        int       `json:"total_items,omitempty"`
	TotalPages        int       `json:"total_pages,omitempty"`
	Gray              string    `json:"gray,omitempty"`
}

type ArticleResult struct {
	GeneratedAt        time.Time `json:"generated_at"`
	DurationSeconds    float64   `json:"duration_seconds"`
	GeneratedFiles     int       `json:"generated_files"`
	GeneratedDetails   int       `json:"generated_details"`
	GeneratedLists     int       `json:"generated_lists"`
	ArticleID          int64     `json:"article_id"`
	ColumnID           int64     `json:"column_id"`
	Output             string    `json:"output"`
	RefreshedColumnIDs []int64   `json:"refreshed_column_ids,omitempty"`
	RefreshedPages     []string  `json:"refreshed_pages,omitempty"`
}

type DeleteArticleResult struct {
	DeletedAt          time.Time `json:"deleted_at"`
	DurationSeconds    float64   `json:"duration_seconds"`
	GeneratedFiles     int       `json:"generated_files"`
	GeneratedDetails   int       `json:"generated_details"`
	GeneratedLists     int       `json:"generated_lists"`
	ArticleID          int64     `json:"article_id"`
	Deleted            bool      `json:"deleted"`
	DeletedPaths       []string  `json:"deleted_paths"`
	RefreshedColumnIDs []int64   `json:"refreshed_column_ids,omitempty"`
	RefreshedPages     []string  `json:"refreshed_pages,omitempty"`
}

type ListResult struct {
	GeneratedAt      time.Time `json:"generated_at"`
	DurationSeconds  float64   `json:"duration_seconds"`
	GeneratedFiles   int       `json:"generated_files"`
	GeneratedDetails int       `json:"generated_details"`
	GeneratedLists   int       `json:"generated_lists"`
	ColumnID         int64     `json:"column_id"`
	TotalItems       int       `json:"total_items"`
	TotalPages       int       `json:"total_pages"`
	PageSize         int       `json:"page_size"`
	Output           string    `json:"output"`
}
