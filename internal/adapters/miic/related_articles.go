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
	if column.TemplateName == g.cfg.Site.PageName {
		columns, err := g.source.FetchPageColumns(ctx, g.cfg.Site.PageName, 0)
		if err != nil {
			return nil, err
		}
		groups := make([][]model.Article, 0, len(columns))
		for _, candidate := range columns {
			if !g.articleSidebarColumn(candidate) {
				continue
			}
			_, items, err := g.source.FetchColumnArticles(ctx, candidate.ID)
			if err != nil {
				return nil, err
			}
			groups = append(groups, items)
		}
		return mergeArticles(groups), nil
	}
	_, items, err := g.source.FetchColumnArticles(ctx, column.ID)
	return items, err
}

func (g *Generator) relatedArticleScopeKey(column model.Column) int64 {
	if column.TemplateName == g.cfg.Site.PageName {
		return 0
	}
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

func (g *Generator) articleSidebarColumn(column model.Column) bool {
	if column.TemplateName != g.cfg.Site.PageName {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(column.Code), "latest") || strings.TrimSpace(column.Name) == strings.TrimSpace(g.cfg.News.Important.Name) {
		return false
	}
	for _, name := range g.cfg.News.ExcludeColumns {
		if strings.TrimSpace(column.Name) == strings.TrimSpace(name) {
			return false
		}
	}
	return true
}
