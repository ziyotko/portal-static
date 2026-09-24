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

	"portal-static/internal/contracts"
)

const maxTemplateSourceBytes = 2 << 20

var templateKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// TemplateBinding maps one adapter role to one CMS template. At least one
// stable selector (ID, Code or Name) is required in addition to Type. Code is
// preferred for deployment configuration; Name remains supported for existing
// installations whose template codes have not yet been standardized.
type TemplateBinding struct {
	Key  string
	ID   int64
	Code string
	Name string
	Type string
}

type BoundTemplate struct {
	Key       string
	ID        int64
	Name      string
	Code      string
	Type      string
	RoutePath string
	Status    int
	Source    string
	Layout    string
}

// LoadBoundTemplates reads Template directly. The removed Page entity is not
// consulted. Every selector must resolve to exactly one enabled template.
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
		binding.Code = strings.TrimSpace(binding.Code)
		binding.Name = strings.TrimSpace(binding.Name)
		binding.Type = strings.TrimSpace(binding.Type)
		if !templateKeyPattern.MatchString(binding.Key) {
			return nil, fmt.Errorf("template binding key %q is invalid", binding.Key)
		}
		if _, exists := keys[binding.Key]; exists {
			return nil, fmt.Errorf("duplicate template binding key %q", binding.Key)
		}
		keys[binding.Key] = struct{}{}
		if binding.ID <= 0 && binding.Code == "" && binding.Name == "" {
			return nil, fmt.Errorf("template binding %q requires id, code or name", binding.Key)
		}
		if binding.Type == "" {
			return nil, fmt.Errorf("template binding %q requires type", binding.Key)
		}
		normalized[index] = binding
	}

	rows, err := s.QueryContext(ctx, `SELECT id,name,COALESCE(code,''),type,COALESCE(route_path,''),status,COALESCE(source_code,''),COALESCE(layout,'')
FROM {{schema}}.template
ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("load database templates: %w", err)
	}
	defer rows.Close()
	records := make([]BoundTemplate, 0, len(bindings))
	for rows.Next() {
		var record BoundTemplate
		if err := rows.Scan(&record.ID, &record.Name, &record.Code, &record.Type, &record.RoutePath, &record.Status, &record.Source, &record.Layout); err != nil {
			return nil, fmt.Errorf("scan database templates: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read database templates: %w", err)
	}

	result := make(map[string]BoundTemplate, len(bindings))
	for _, binding := range normalized {
		selectorMatches := make([]BoundTemplate, 0, 2)
		matches := make([]BoundTemplate, 0, 2)
		for _, candidate := range records {
			if binding.ID > 0 && candidate.ID != binding.ID {
				continue
			}
			if binding.Code != "" && candidate.Code != binding.Code {
				continue
			}
			if binding.Name != "" && candidate.Name != binding.Name {
				continue
			}
			selectorMatches = append(selectorMatches, candidate)
			if candidate.Type == binding.Type {
				matches = append(matches, candidate)
			}
		}
		selector := templateSelector(binding)
		if len(selectorMatches) == 0 {
			return nil, fmt.Errorf("%w: database template %q is unavailable: %s was not found with type %q", contracts.ErrTemplateNotFound, binding.Key, selector, binding.Type)
		}
		if len(matches) == 0 {
			actualTypes := make([]string, 0, len(selectorMatches))
			for _, record := range selectorMatches {
				actualTypes = append(actualTypes, record.Type)
			}
			return nil, fmt.Errorf("database template %q type is %q, want %q", binding.Key, strings.Join(actualTypes, ","), binding.Type)
		}
		if len(matches) != 1 {
			return nil, fmt.Errorf("%w: database template %q is ambiguous: %d templates match %s", contracts.ErrTemplateNotUnique, binding.Key, len(matches), selector)
		}
		record := matches[0]
		if record.Status != 1 {
			return nil, fmt.Errorf("database template %q (%s) is disabled", binding.Key, record.Name)
		}
		if strings.TrimSpace(record.Source) == "" {
			return nil, fmt.Errorf("database template %q (%s) has empty source_code", binding.Key, record.Name)
		}
		if len(record.Source) > maxTemplateSourceBytes {
			return nil, fmt.Errorf("database template %q exceeds %d bytes", binding.Key, maxTemplateSourceBytes)
		}
		record.Key = binding.Key
		result[binding.Key] = record
	}
	return result, nil
}

func templateSelector(binding TemplateBinding) string {
	parts := make([]string, 0, 3)
	if binding.ID > 0 {
		parts = append(parts, fmt.Sprintf("id=%d", binding.ID))
	}
	if binding.Code != "" {
		parts = append(parts, fmt.Sprintf("code=%q", binding.Code))
	}
	if binding.Name != "" {
		parts = append(parts, fmt.Sprintf("name=%q", binding.Name))
	}
	return strings.Join(parts, ", ")
}

// LoadTemplatesByType returns all enabled templates of a type. It is used for
// special pages, where each template is a separately addressable page.
func (s *Store) LoadTemplatesByType(ctx context.Context, templateType string) ([]BoundTemplate, error) {
	templateType = strings.TrimSpace(templateType)
	if templateType == "" {
		return nil, errors.New("template type is required")
	}
	rows, err := s.QueryContext(ctx, `SELECT id,name,COALESCE(code,''),type,COALESCE(route_path,''),status,COALESCE(source_code,''),COALESCE(layout,'')
FROM {{schema}}.template WHERE type=? AND status=1 ORDER BY id`, templateType)
	if err != nil {
		return nil, fmt.Errorf("load %s templates: %w", templateType, err)
	}
	defer rows.Close()
	var records []BoundTemplate
	for rows.Next() {
		var record BoundTemplate
		if err := rows.Scan(&record.ID, &record.Name, &record.Code, &record.Type, &record.RoutePath, &record.Status, &record.Source, &record.Layout); err != nil {
			return nil, fmt.Errorf("scan %s template: %w", templateType, err)
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Store) LoadTemplateByID(ctx context.Context, id int64, templateType string) (BoundTemplate, error) {
	if id <= 0 {
		return BoundTemplate{}, errors.New("template id must be positive")
	}
	var record BoundTemplate
	err := s.QueryRowContext(ctx, `SELECT id,name,COALESCE(code,''),type,COALESCE(route_path,''),status,COALESCE(source_code,''),COALESCE(layout,'')
FROM {{schema}}.template WHERE id=?`, id).Scan(&record.ID, &record.Name, &record.Code, &record.Type, &record.RoutePath, &record.Status, &record.Source, &record.Layout)
	if err != nil {
		return BoundTemplate{}, fmt.Errorf("load template %d: %w", id, err)
	}
	if record.Type != templateType {
		return BoundTemplate{}, fmt.Errorf("template %d type is %q, want %q", id, record.Type, templateType)
	}
	if record.Status != 1 {
		return BoundTemplate{}, fmt.Errorf("template %d is disabled", id)
	}
	if strings.TrimSpace(record.Source) == "" {
		return BoundTemplate{}, fmt.Errorf("template %d has empty source_code", id)
	}
	if len(record.Source) > maxTemplateSourceBytes {
		return BoundTemplate{}, fmt.Errorf("template %d exceeds %d bytes", id, maxTemplateSourceBytes)
	}
	return record, nil
}

// MaterializeTemplates validates database template syntax and writes an
// ephemeral, private template bundle for the existing renderers.
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
			return fail(fmt.Errorf("parse database template %q (%s): %w", key, record.Name, err))
		}
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, []byte(record.Source), 0o600); err != nil {
			return fail(fmt.Errorf("write runtime template %q: %w", key, err))
		}
		paths[key] = path
	}
	return paths, cleanup, nil
}
