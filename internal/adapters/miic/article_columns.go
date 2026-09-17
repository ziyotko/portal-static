package miic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	nethtml "golang.org/x/net/html"
	"portal-static/internal/adapters/miic/model"
)

// Resolve before replacing any output: a removed publication relationship may
// survive only in an old list or in the pending state of an interrupted refresh.
func (g *Generator) resolveArticleRefreshColumns(ctx context.Context, root string, articleID int64) ([]model.Column, error) {
	current, err := g.source.FetchArticleRelatedColumns(ctx, articleID)
	if err != nil {
		return nil, fmt.Errorf("load article related columns: %w", err)
	}
	pending, err := os.ReadFile(articleRefreshStatePath(root, articleID))
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
	all, err := g.source.FetchColumns(ctx)
	if err != nil {
		return nil, err
	}
	// Full-site releases embed news directly in main pages without producing
	// lists. If only a main-page reference survives, refresh that page's columns.
	previousPages := make(map[string]bool)
	for _, page := range []string{"news", "business", "platforms", "about"} {
		found, err := listReferencesArticle(ctx, root, filepath.Join(root, page+".html"), articleID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect previous page %s: %w", page, err)
		}
		previousPages[page] = found
	}
	known := make(map[int64]bool, len(current))
	for _, column := range current {
		known[column.ID] = true
	}
	for _, column := range all {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if known[column.ID] {
			continue
		}
		dir := filepath.Join(root, "list", strconv.FormatInt(column.ID, 10))
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
			found, err := listReferencesArticle(ctx, root, filepath.Join(dir, entry.Name()), articleID)
			if err != nil {
				return nil, fmt.Errorf("inspect previous list %d: %w", column.ID, err)
			}
			if found {
				current = append(current, column)
				break
			}
		}
	}
	knownPages := make(map[string]bool)
	for _, column := range current {
		if page, ok := NormalizePageName(column.PageName); ok {
			knownPages[page] = true
		}
	}
	for _, column := range all {
		page, ok := NormalizePageName(column.PageName)
		if ok && previousPages[page] && !knownPages[page] {
			current = append(current, column)
		}
	}
	current = mergeColumns(current)
	data, err := json.Marshal(current)
	if err != nil {
		return nil, err
	}
	if err := publishFile(articleRefreshStatePath(root, articleID), data); err != nil {
		return nil, fmt.Errorf("save pending article columns: %w", err)
	}
	return current, nil
}

func articleRefreshStatePath(root string, articleID int64) string {
	return filepath.Join(root, ".article-refresh", strconv.FormatInt(articleID, 10)+".json")
}

func clearArticleRefreshColumns(root string, articleID int64) error {
	err := os.Remove(articleRefreshStatePath(root, articleID))
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
			parsed, err := url.Parse(strings.TrimSpace(attr.Val))
			if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Path == "" || strings.Contains(parsed.Path, "\\") {
				continue
			}
			var target string
			if strings.HasPrefix(parsed.Path, "/") {
				target = filepath.Join(root, filepath.FromSlash(strings.TrimLeft(parsed.Path, "/")))
			} else {
				target = filepath.Join(filepath.Dir(source), filepath.FromSlash(parsed.Path))
			}
			relative, err := filepath.Rel(filepath.Join(root, "article"), target)
			if err != nil {
				continue
			}
			parts := strings.Split(filepath.ToSlash(relative), "/")
			if len(parts) == 3 && parts[0] != ".." && parts[2] == id+".html" {
				return true, nil
			}
		}
	}
}
