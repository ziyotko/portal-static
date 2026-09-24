package portalcms

import (
	"context"
	"fmt"
	"path/filepath"

	"portal-static/internal/core/httpapi"
)

// WithRouteAliases decorates successful legacy generation operations with the
// files addressed by dia-platform's current template and column route_path
// values. Existing files remain in place for backwards-compatible URLs.
func WithRouteAliases(operations httpapi.Operations, publisher RoutePublisher, mainFiles map[string]string) httpapi.Operations {
	publishMains := func(ctx context.Context) error {
		for key, legacyFile := range mainFiles {
			if err := publisher.PublishMain(ctx, key, legacyFile); err != nil {
				return fmt.Errorf("publish %s route alias: %w", key, err)
			}
		}
		return nil
	}
	publishAll := func(ctx context.Context) error {
		if err := publishMains(ctx); err != nil {
			return err
		}
		if err := publisher.PublishAllLists(ctx); err != nil {
			return fmt.Errorf("publish list route aliases: %w", err)
		}
		if err := publisher.PublishAllArticles(ctx); err != nil {
			return fmt.Errorf("publish article route aliases: %w", err)
		}
		return nil
	}

	generateSite := operations.GenerateSite
	operations.GenerateSite = func(ctx context.Context) (any, error) {
		result, err := generateSite(ctx)
		if err == nil {
			err = publishAll(ctx)
		}
		return result, err
	}
	generatePages := operations.GeneratePages
	operations.GeneratePages = func(ctx context.Context) (any, error) {
		result, err := generatePages(ctx)
		if err == nil {
			err = publishMains(ctx)
		}
		return result, err
	}
	generatePage := operations.GeneratePage
	operations.GeneratePage = func(ctx context.Context, name string) (any, error) {
		result, err := generatePage(ctx, name)
		if err != nil {
			return result, err
		}
		key, ok := operations.NormalizePageName(name)
		if !ok {
			return result, fmt.Errorf("cannot resolve generated page %q", name)
		}
		legacyFile, ok := mainFiles[key]
		if !ok {
			return result, fmt.Errorf("legacy output for page %q is not configured", key)
		}
		return result, publisher.PublishMain(ctx, key, legacyFile)
	}
	generateAllLists := operations.GenerateAllLists
	operations.GenerateAllLists = func(ctx context.Context) (any, error) {
		result, err := generateAllLists(ctx)
		if err == nil {
			err = publisher.PublishAllLists(ctx)
		}
		return result, err
	}
	generateList := operations.GenerateList
	operations.GenerateList = func(ctx context.Context, id int64) (any, error) {
		result, err := generateList(ctx, id)
		if err == nil {
			err = publisher.PublishList(ctx, ResultInt64(result, "column_id"), ResultString(result, "output"))
		}
		return result, err
	}
	generateListByName := operations.GenerateListByName
	operations.GenerateListByName = func(ctx context.Context, name string) (any, error) {
		result, err := generateListByName(ctx, name)
		if err == nil {
			err = publisher.PublishList(ctx, ResultInt64(result, "column_id"), ResultString(result, "output"))
		}
		return result, err
	}
	generateAllArticles := operations.GenerateAllArticles
	operations.GenerateAllArticles = func(ctx context.Context) (any, error) {
		result, err := generateAllArticles(ctx)
		if err == nil {
			err = publisher.PublishAllArticles(ctx)
		}
		return result, err
	}
	generateArticle := operations.GenerateArticle
	operations.GenerateArticle = func(ctx context.Context, id int64) (any, error) {
		result, err := generateArticle(ctx, id)
		if err == nil {
			err = publisher.PublishArticle(ctx, id, ResultString(result, "output"))
		}
		return result, err
	}
	deleteArticle := operations.DeleteArticle
	operations.DeleteArticle = func(ctx context.Context, id int64) (any, error) {
		result, err := deleteArticle(ctx, id)
		if err == nil {
			err = publisher.DeleteArticle(ctx, id)
		}
		return result, err
	}
	generateRelated := operations.GenerateArticleRelated
	operations.GenerateArticleRelated = func(ctx context.Context, id int64) (any, error) {
		result, err := generateRelated(ctx, id)
		if err == nil {
			err = publisher.PublishArticle(ctx, id, ResultString(result, "output"))
		}
		if err == nil {
			err = publisher.PublishAllLists(ctx)
		}
		if err == nil {
			err = publishMains(ctx)
		}
		return result, err
	}
	deleteRelated := operations.DeleteArticleRelated
	operations.DeleteArticleRelated = func(ctx context.Context, id int64) (any, error) {
		result, err := deleteRelated(ctx, id)
		if err == nil {
			err = publisher.DeleteArticle(ctx, id)
		}
		if err == nil {
			err = publisher.PublishAllLists(ctx)
		}
		if err == nil {
			err = publishMains(ctx)
		}
		return result, err
	}

	return operations
}

func LegacyMainFiles(names ...string) map[string]string {
	files := make(map[string]string, len(names))
	for _, name := range names {
		files[name] = name + ".html"
	}
	return files
}

func LegacyCAAMMainFiles() map[string]string {
	files := LegacyMainFiles("about", "work", "stats", "members", "party")
	files["home"] = filepath.Base("index.html")
	return files
}
