package portalcms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"portal-static/internal/contracts"
)

// RoutePublisher creates compatibility copies at the paths advertised by the
// current dia-platform route_path rules while retaining the portals' existing
// static file layout.
type RoutePublisher struct {
	Store       *Store
	AllowedRoot string
	Routes      map[string]string
}

func (p RoutePublisher) PublishMain(ctx context.Context, key, legacyFile string) error {
	route := strings.TrimSpace(p.Routes[key])
	if route == "" {
		return fmt.Errorf("template route_path for %q is empty", key)
	}
	root, err := p.root(ctx)
	if err != nil {
		return err
	}
	destination, err := p.routeDestination(root, route)
	if err != nil {
		return err
	}
	return copyAlias(filepath.Join(root, legacyFile), destination)
}

func (p RoutePublisher) PublishArticle(ctx context.Context, articleID int64, source string) error {
	if articleID <= 0 {
		return errors.New("article id must be positive")
	}
	root, err := p.root(ctx)
	if err != nil {
		return err
	}
	base := strings.TrimRight(strings.TrimSpace(p.Routes["article"]), "/")
	if base == "" {
		return errors.New("detail template route_path is empty")
	}
	destination, err := p.routeDestination(root, base+"/"+strconv.FormatInt(articleID, 10)+".html")
	if err != nil {
		return err
	}
	return copyAlias(source, destination)
}

func (p RoutePublisher) DeleteArticle(ctx context.Context, articleID int64) error {
	root, err := p.root(ctx)
	if err != nil {
		return err
	}
	base := strings.TrimRight(strings.TrimSpace(p.Routes["article"]), "/")
	if base == "" {
		return errors.New("detail template route_path is empty")
	}
	destination, err := p.routeDestination(root, base+"/"+strconv.FormatInt(articleID, 10)+".html")
	if err != nil {
		return err
	}
	if err := os.Remove(destination); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (p RoutePublisher) PublishAllArticles(ctx context.Context) error {
	root, err := p.root(ctx)
	if err != nil {
		return err
	}
	legacyRoot := filepath.Join(root, "article")
	return filepath.WalkDir(legacyRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if errors.Is(walkErr, fs.ErrNotExist) {
			return nil
		}
		if walkErr != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".html") {
			return walkErr
		}
		id, parseErr := strconv.ParseInt(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), 10, 64)
		if parseErr != nil || id <= 0 {
			return nil
		}
		return p.PublishArticle(ctx, id, path)
	})
}

func (p RoutePublisher) PublishList(ctx context.Context, columnID int64, legacyDirectory string) error {
	if p.Store == nil {
		return errors.New("portal CMS store is nil")
	}
	var route string
	err := p.Store.QueryRowContext(ctx, "SELECT COALESCE(route_path,'') FROM {{schema}}.`column` WHERE id=? AND status=1", columnID).Scan(&route)
	if err != nil {
		return fmt.Errorf("load column %d route_path: %w", columnID, err)
	}
	root, err := p.root(ctx)
	if err != nil {
		return err
	}
	joined := joinRoutePath(route, p.Routes["list"])
	destination, err := p.routeDestination(root, joined)
	if err != nil {
		return err
	}
	return copyAlias(filepath.Join(legacyDirectory, "1.html"), destination)
}

func (p RoutePublisher) PublishAllLists(ctx context.Context) error {
	rows, err := p.Store.QueryContext(ctx, "SELECT id,COALESCE(route_path,'') FROM {{schema}}.`column` WHERE status=1 ORDER BY id")
	if err != nil {
		return err
	}
	defer rows.Close()
	root, err := p.root(ctx)
	if err != nil {
		return err
	}
	type item struct {
		id    int64
		route string
	}
	var items []item
	seen := make(map[string]int64)
	for rows.Next() {
		var current item
		if err := rows.Scan(&current.id, &current.route); err != nil {
			return err
		}
		destination, err := p.routeDestination(root, joinRoutePath(current.route, p.Routes["list"]))
		if err != nil {
			return err
		}
		key := strings.ToLower(filepath.Clean(destination))
		if previous, exists := seen[key]; exists {
			return fmt.Errorf("columns %d and %d resolve to the same static route", previous, current.id)
		}
		seen[key] = current.id
		items = append(items, current)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, current := range items {
		legacy := filepath.Join(root, "list", strconv.FormatInt(current.id, 10))
		source := filepath.Join(legacy, "1.html")
		if _, err := os.Stat(source); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		destination, err := p.routeDestination(root, joinRoutePath(current.route, p.Routes["list"]))
		if err != nil {
			return err
		}
		if err := copyAlias(source, destination); err != nil {
			return err
		}
	}
	return nil
}

func (p RoutePublisher) root(ctx context.Context) (string, error) {
	root := strings.TrimSpace(contracts.OptionsFrom(ctx).OutputPath)
	if root == "" {
		root = p.AllowedRoot
	}
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	allowed, err := filepath.Abs(filepath.Clean(p.AllowedRoot))
	if err != nil {
		return "", err
	}
	if err := requirePathWithin(allowed, root); err != nil {
		return "", contracts.ErrInvalidOutputPath
	}
	return root, nil
}

func (p RoutePublisher) routeDestination(root, route string) (string, error) {
	rel, err := routeOutputFile(route)
	if err != nil {
		return "", err
	}
	destination := filepath.Join(root, filepath.FromSlash(rel))
	if err := requirePathWithin(root, destination); err != nil {
		return "", err
	}
	if err := (TopicService{AllowedRoot: p.AllowedRoot}).ensureSafeDestination(root, destination); err != nil {
		return "", err
	}
	return destination, nil
}

func joinRoutePath(parts ...string) string {
	result := ""
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(strings.ReplaceAll(part, "\\", "/")), "/")
		if part != "" {
			result += "/" + part
		}
	}
	return result
}

func copyAlias(source, destination string) error {
	if filepath.Clean(source) == filepath.Clean(destination) {
		return nil
	}
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open route source %s: %w", source, err)
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(destination), ".route-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := io.Copy(temp, input); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, destination); err != nil {
		return fmt.Errorf("publish route alias %s: %w", destination, err)
	}
	return nil
}

func ResultString(result any, field string) string {
	values := resultMap(result)
	value, _ := values[field].(string)
	return value
}

func ResultInt64(result any, field string) int64 {
	values := resultMap(result)
	switch value := values[field].(type) {
	case float64:
		return int64(value)
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
	}
	return 0
}

func resultMap(result any) map[string]any {
	data, err := json.Marshal(result)
	if err != nil {
		return nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	var values map[string]any
	_ = decoder.Decode(&values)
	return values
}
