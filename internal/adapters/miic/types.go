package miic

import (
	"context"
	"html/template"

	"portal-static/internal/cms/model"
	"portal-static/internal/contracts"
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

type GenerationResult = contracts.GenerationResult
type ArticleResult = contracts.ArticleResult
type DeleteArticleResult = contracts.DeleteArticleResult
type ListResult = contracts.ListResult
