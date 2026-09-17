package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	nethtml "golang.org/x/net/html"
	"portal-static/internal/adapters/caam/model"
)

// Existing lists retain the previous relationships when the CMS has already
// moved the article or removed its publication rows. Inspect the requested
// output root before replacing any files, so retries can still find old links.
func (g *SiteGenerator) resolveArticleRefreshColumns(ctx context.Context, articleID int64) ([]model.Column, error) {
	current, err := g.homeSource.FetchArticleRelatedColumns(ctx, articleID)
	if err != nil {
		return nil, err
	}
	pending, err := os.ReadFile(g.articleRefreshStatePath(articleID))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read pending article columns: %w", err)
	}
	if err == nil {
		var columns []model.Column
		if err := json.Unmarshal(pending, &columns); err != nil {
			return nil, fmt.Errorf("decode pending article columns: %w", err)
		}
		current = mergeColumns(current, columns)
	}
	allColumns, err := g.homeSource.FetchColumns(ctx)
	if err != nil {
		return nil, err
	}
	known := make(map[int64]bool, len(current))
	for _, column := range current {
		known[column.ID] = true
	}
	for _, column := range allColumns {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if known[column.ID] {
			continue
		}
		dir := filepath.Join(g.cfg.Site.DistRoot, "list", strconv.FormatInt(column.ID, 10))
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read previous list %d: %w", column.ID, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".html") {
				continue
			}
			found, err := listReferencesArticle(ctx, g.cfg.Site.DistRoot, filepath.Join(dir, entry.Name()), articleID)
			if err != nil {
				return nil, fmt.Errorf("inspect previous list %d: %w", column.ID, err)
			}
			if found {
				current = append(current, column)
				known[column.ID] = true
				break
			}
		}
	}
	current = mergeColumns(current)
	if err := g.saveArticleRefreshColumns(articleID, current); err != nil {
		return nil, err
	}
	return current, nil
}

func (g *SiteGenerator) articleRefreshStatePath(articleID int64) string {
	return filepath.Join(g.cfg.Site.DistRoot, ".article-refresh", strconv.FormatInt(articleID, 10)+".json")
}

// Keep the resolved set until every list and main page has been refreshed.
// Otherwise a retry could lose old columns whose lists were already replaced.
func (g *SiteGenerator) saveArticleRefreshColumns(articleID int64, columns []model.Column) error {
	path := g.articleRefreshStatePath(articleID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(columns)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".pending-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	_, writeErr := temp.Write(data)
	closeErr := temp.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(temp.Name(), path)
}

func (g *SiteGenerator) clearArticleRefreshColumns(articleID int64) error {
	err := os.Remove(g.articleRefreshStatePath(articleID))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func listReferencesArticle(ctx context.Context, root, source string, articleID int64) (bool, error) {
	file, err := os.Open(source)
	if err != nil {
		return false, err
	}
	defer file.Close()
	tokenizer := nethtml.NewTokenizer(file)
	id := strconv.FormatInt(articleID, 10)
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		kind := tokenizer.Next()
		if kind == nethtml.ErrorToken {
			if errors.Is(tokenizer.Err(), io.EOF) {
				return false, nil
			}
			return false, tokenizer.Err()
		}
		if kind != nethtml.StartTagToken && kind != nethtml.SelfClosingTagToken {
			continue
		}
		token := tokenizer.Token()
		if token.Data != "a" {
			continue
		}
		for _, attr := range token.Attr {
			if attr.Key != "href" {
				continue
			}
			target, owned, err := generatedLinkTarget(root, source, attr.Val, nil)
			if err != nil {
				return false, err
			}
			if !owned {
				continue
			}
			relative, err := filepath.Rel(filepath.Join(root, "article"), target)
			if err != nil {
				return false, err
			}
			parts := strings.Split(filepath.ToSlash(relative), "/")
			if len(parts) == 3 && parts[0] != ".." && parts[2] == id+".html" ||
				len(parts) == 2 && parts[0] == id && parts[1] == "index.html" {
				return true, nil
			}
		}
	}
}
