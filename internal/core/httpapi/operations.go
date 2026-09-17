package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"portal-static/internal/contracts"
)

type Operation func(context.Context) (any, error)
type PageOperation func(context.Context, string) (any, error)
type ListOperation func(context.Context, int64) (any, error)
type NamedListOperation func(context.Context, string) (any, error)
type ArticleOperation func(context.Context, int64) (any, error)

type Operations struct {
	GenerateSite           Operation
	GeneratePages          Operation
	GenerateAllLists       Operation
	GenerateAllArticles    Operation
	GeneratePage           PageOperation
	GenerateList           ListOperation
	GenerateListByName     NamedListOperation
	GenerateArticle        ArticleOperation
	DeleteArticle          ArticleOperation
	GenerateArticleRelated ArticleOperation
	DeleteArticleRelated   ArticleOperation
	NormalizePageName      func(string) (string, bool)
	PageNameError          string
	ValidateOutputPath     func(string) error
	ClassifyError          func(error) (int, string, bool)
}

func OperationsForGenerator(generator contracts.Generator, normalize func(string) (string, bool), pageNameError string) Operations {
	operations := Operations{
		GenerateSite:           func(ctx context.Context) (any, error) { return generator.GenerateSite(ctx) },
		GeneratePages:          func(ctx context.Context) (any, error) { return generator.GeneratePages(ctx) },
		GenerateAllLists:       func(ctx context.Context) (any, error) { return generator.GenerateAllLists(ctx) },
		GenerateAllArticles:    func(ctx context.Context) (any, error) { return generator.GenerateAllArticles(ctx) },
		GeneratePage:           func(ctx context.Context, name string) (any, error) { return generator.GeneratePage(ctx, name) },
		GenerateList:           func(ctx context.Context, id int64) (any, error) { return generator.GenerateList(ctx, id) },
		GenerateListByName:     func(ctx context.Context, name string) (any, error) { return generator.GenerateListByName(ctx, name) },
		GenerateArticle:        func(ctx context.Context, id int64) (any, error) { return generator.GenerateArticle(ctx, id) },
		DeleteArticle:          func(ctx context.Context, id int64) (any, error) { return generator.DeleteArticle(ctx, id) },
		GenerateArticleRelated: func(ctx context.Context, id int64) (any, error) { return generator.GenerateArticleRelated(ctx, id) },
		DeleteArticleRelated:   func(ctx context.Context, id int64) (any, error) { return generator.DeleteArticleRelated(ctx, id) },
		NormalizePageName:      normalize,
		PageNameError:          pageNameError,
	}
	if validator, ok := generator.(interface{ ValidateOutputPath(string) error }); ok {
		operations.ValidateOutputPath = validator.ValidateOutputPath
	}
	return operations
}

func defaultErrorClassification(err error) (int, string, bool) {
	switch {
	case errors.Is(err, contracts.ErrBusy):
		return http.StatusConflict, err.Error(), true
	case errors.Is(err, contracts.ErrArticleStillPublished):
		return http.StatusConflict, "文章仍处于可发布状态，请先在数据库下架", true
	case errors.Is(err, contracts.ErrColumnNotUnique), errors.Is(err, contracts.ErrPageNotUnique):
		return http.StatusConflict, err.Error(), true
	case errors.Is(err, contracts.ErrColumnNotFound), errors.Is(err, contracts.ErrPageNotFound), errors.Is(err, contracts.ErrArticleNotPublished):
		return http.StatusNotFound, err.Error(), true
	case errors.Is(err, contracts.ErrInvalidOutputPath), strings.Contains(err.Error(), "output path"), strings.Contains(err.Error(), "unsupported page"):
		return http.StatusBadRequest, err.Error(), true
	default:
		return 0, "", false
	}
}
