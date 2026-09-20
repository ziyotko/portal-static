package portalcms

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxTemplateSourceBytes = 2 << 20

var templateKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// TemplateBinding identifies the active database template used for one
// adapter template role. PageName is used for named top-level pages; when it
// is empty, PageType must identify exactly one active page (for example the
// shared column or detail page).
type TemplateBinding struct {
	Key          string
	PageName     string
	PageType     string
	TemplateType string
}

type BoundTemplate struct {
	Key          string
	PageID       int64
	PageName     string
	PageType     string
	TemplateID   int64
	TemplateName string
	TemplateType string
	Source       string
}

// LoadBoundTemplates resolves page.template_id and returns the active
// template source stored in the Portal CMS. Production adapters use this as
// their only template source; repository files are reserved for preview.
func (s *Store) LoadBoundTemplates(ctx context.Context, bindings []TemplateBinding) (map[string]BoundTemplate, error) {
	if s == nil {
		return nil, errors.New("portal CMS store is nil")
	}
	if len(bindings) == 0 {
		return nil, errors.New("at least one template binding is required")
	}
	normalized := make([]TemplateBinding, len(bindings))
	keys := make(map[string]struct{}, len(bindings))
	for index, binding := range bindings {
		binding.Key = strings.TrimSpace(binding.Key)
		binding.PageName = strings.TrimSpace(binding.PageName)
		binding.PageType = strings.TrimSpace(binding.PageType)
		binding.TemplateType = strings.TrimSpace(binding.TemplateType)
		if !templateKeyPattern.MatchString(binding.Key) {
			return nil, fmt.Errorf("template binding key %q is invalid", binding.Key)
		}
		if _, exists := keys[binding.Key]; exists {
			return nil, fmt.Errorf("duplicate template binding key %q", binding.Key)
		}
		keys[binding.Key] = struct{}{}
		if binding.PageName == "" && binding.PageType == "" {
			return nil, fmt.Errorf("template binding %q requires page name or type", binding.Key)
		}
		if binding.TemplateType == "" {
			return nil, fmt.Errorf("template binding %q requires template type", binding.Key)
		}
		normalized[index] = binding
	}

	query := `SELECT p.id,p.name,p.page_type,t.id,t.name,t.type,COALESCE(t.source_code,'')
FROM {{schema}}.page p
INNER JOIN {{schema}}.template t ON t.id=p.template_id AND t.status=1 AND t.deleted_at IS NULL
WHERE p.status=1 AND p.deleted_at IS NULL
ORDER BY p.id`
	rows, err := s.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load database templates: %w", err)
	}
	defer rows.Close()
	records := make([]BoundTemplate, 0, len(bindings))
	for rows.Next() {
		var record BoundTemplate
		if err := rows.Scan(
			&record.PageID, &record.PageName, &record.PageType,
			&record.TemplateID, &record.TemplateName, &record.TemplateType, &record.Source,
		); err != nil {
			return nil, fmt.Errorf("scan database templates: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read database templates: %w", err)
	}

	result := make(map[string]BoundTemplate, len(bindings))
	for _, binding := range normalized {
		matches := make([]BoundTemplate, 0, 2)
		for _, candidate := range records {
			if binding.PageName != "" && candidate.PageName != binding.PageName {
				continue
			}
			if binding.PageType != "" && candidate.PageType != binding.PageType {
				continue
			}
			matches = append(matches, candidate)
		}
		if len(matches) == 0 {
			selector := "page type " + binding.PageType
			if binding.PageName != "" {
				selector = "page " + binding.PageName
			}
			return nil, fmt.Errorf("database template %q is unavailable: active %s is missing, unbound, or uses an inactive template", binding.Key, selector)
		}
		if len(matches) != 1 {
			return nil, fmt.Errorf("database template %q is ambiguous: %d active pages match", binding.Key, len(matches))
		}
		record := matches[0]
		record.Key = binding.Key
		if record.TemplateType != binding.TemplateType {
			return nil, fmt.Errorf(
				"database template %q type is %q, want %q", binding.Key, record.TemplateType, binding.TemplateType,
			)
		}
		if strings.TrimSpace(record.Source) == "" {
			return nil, fmt.Errorf("database template %q (%s) has empty source_code", binding.Key, record.TemplateName)
		}
		if len(record.Source) > maxTemplateSourceBytes {
			return nil, fmt.Errorf("database template %q exceeds %d bytes", binding.Key, maxTemplateSourceBytes)
		}
		result[binding.Key] = record
	}
	return result, nil
}

// MaterializeTemplates validates database template syntax and writes an
// ephemeral, private template bundle. Existing generators can keep using
// html/template.ParseFiles while production remains database-authoritative.
// The returned cleanup must be called after the generator has finished using
// the bundle. Production adapters create a fresh bundle for every operation.
func MaterializeTemplates(records map[string]BoundTemplate) (map[string]string, func() error, error) {
	if len(records) == 0 {
		return nil, nil, errors.New("no database templates to materialize")
	}
	dir, err := os.MkdirTemp("", "portal-static-templates-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create runtime template directory: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(dir) }
	fail := func(err error) (map[string]string, func() error, error) {
		_ = cleanup()
		return nil, nil, err
	}

	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	paths := make(map[string]string, len(records))
	for _, key := range keys {
		record := records[key]
		if record.Key != key || !templateKeyPattern.MatchString(key) {
			return fail(fmt.Errorf("database template key %q is invalid", key))
		}
		filename := key + ".html.tmpl"
		if _, err := template.New(filename).Parse(record.Source); err != nil {
			return fail(fmt.Errorf("parse database template %q (%s): %w", key, record.TemplateName, err))
		}
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, []byte(record.Source), 0o600); err != nil {
			return fail(fmt.Errorf("write runtime template %q: %w", key, err))
		}
		paths[key] = path
	}
	return paths, cleanup, nil
}
