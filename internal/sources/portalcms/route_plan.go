package portalcms

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// ValidateRoutePlan rejects cross-type collisions before any route aliases are
// written. Paths are compared case-insensitively because supported production
// deployments include Windows and commonly case-insensitive web roots.
func ValidateRoutePlan(ctx context.Context, store *Store, templates map[string]BoundTemplate) error {
	if store == nil {
		return fmt.Errorf("portal CMS store is nil")
	}
	seen := make(map[string]string)
	add := func(route, owner string) error {
		file, err := routeOutputFile(route)
		if err != nil {
			return fmt.Errorf("%s: %w", owner, err)
		}
		key := strings.ToLower(filepath.Clean(filepath.FromSlash(file)))
		if previous, exists := seen[key]; exists && previous != owner {
			return fmt.Errorf("static route conflict: %s and %s both resolve to %s", previous, owner, file)
		}
		seen[key] = owner
		return nil
	}
	for key, record := range templates {
		if key == "list" || key == "article" {
			continue
		}
		if err := add(record.RoutePath, "template "+key); err != nil {
			return err
		}
	}

	list, ok := templates["list"]
	if !ok {
		return fmt.Errorf("list template route is missing")
	}
	rows, err := store.QueryContext(ctx, "SELECT id,COALESCE(route_path,'') FROM {{schema}}.`column` WHERE status=1 ORDER BY id")
	if err != nil {
		return fmt.Errorf("load column route plan: %w", err)
	}
	for rows.Next() {
		var id int64
		var route string
		if err := rows.Scan(&id, &route); err != nil {
			_ = rows.Close()
			return err
		}
		if err := add(joinRoutePath(route, list.RoutePath), "column "+strconv.FormatInt(id, 10)); err != nil {
			_ = rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	detail, ok := templates["article"]
	if !ok {
		return fmt.Errorf("detail template route is missing")
	}
	rows, err = store.QueryContext(ctx, `
SELECT DISTINCT a.id
FROM {{schema}}.article a
INNER JOIN {{schema}}.article_column_publish acp ON acp.article_id=a.id
INNER JOIN {{schema}}.`+"`column`"+` c
        ON c.id=acp.column_id AND c.status=1 AND c.template_id=acp.template_id
WHERE a.status=1 AND a.audit_status=2 AND a.type IN (1,2)
ORDER BY a.id`)
	if err != nil {
		return fmt.Errorf("load article route plan: %w", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		route := joinRoutePath(detail.RoutePath, strconv.FormatInt(id, 10)+".html")
		if err := add(route, "article "+strconv.FormatInt(id, 10)); err != nil {
			_ = rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	rows, err = store.QueryContext(ctx, "SELECT id,COALESCE(route_path,'') FROM {{schema}}.template WHERE type='special' AND status=1 ORDER BY id")
	if err != nil {
		return fmt.Errorf("load special route plan: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var route string
		if err := rows.Scan(&id, &route); err != nil {
			return err
		}
		if err := add(route, "special template "+strconv.FormatInt(id, 10)); err != nil {
			return err
		}
	}
	return rows.Err()
}
