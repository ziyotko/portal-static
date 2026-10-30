package miic

import (
	"context"
	"strings"

	"portal-static/internal/adapters/miic/model"
)

const relatedArticleLimit = 6

func (g *Generator) loadRelatedArticleViews(ctx context.Context, column model.Column, currentID int64) ([]ArticleView, error) {
	items, err := g.loadRelatedArticleItems(ctx, column)
	if err != nil {
		return nil, err
	}
	return g.relatedArticleViews(column, currentID, items), nil
}

func (g *Generator) loadRelatedArticleItems(ctx context.Context, column model.Column) ([]model.Article, error) {
	_, items, err := g.source.FetchColumnArticles(ctx, column.ID)
	return items, err
}

func (g *Generator) relatedArticleScopeKey(column model.Column) int64 {
	return column.ID
}

func (g *Generator) relatedArticleViews(column model.Column, currentID int64, items []model.Article) []ArticleView {
	views := make([]ArticleView, 0, relatedArticleLimit)
	for _, item := range items {
		if item.ID == currentID {
			continue
		}
		view := makeViewWithResolver(g.cfg, g.media, column.Name, item)
		if view.Href == "" {
			continue
		}
		if !view.External {
			view.Href = "../../../" + strings.TrimPrefix(view.Href, "./")
		}
		views = append(views, view)
		if len(views) == relatedArticleLimit {
			break
		}
	}
	return views
}
