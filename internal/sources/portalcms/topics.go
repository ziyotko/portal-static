package portalcms

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"portal-static/internal/contracts"
)

type TopicService struct {
	Store       *Store
	AllowedRoot string
	Location    *time.Location
}

type topicTemplateData struct {
	GeneratedAt string
	Template    BoundTemplate
}

func (s TopicService) GenerateAll(ctx context.Context) (contracts.GenerationResult, error) {
	started := time.Now()
	records, err := s.Store.LoadTemplatesByType(ctx, "special")
	if err != nil {
		return contracts.GenerationResult{}, err
	}
	paths := make(map[string]int64, len(records))
	for _, record := range records {
		path, err := s.outputPath(ctx, record.RoutePath)
		if err != nil {
			return contracts.GenerationResult{}, fmt.Errorf("special template %d: %w", record.ID, err)
		}
		key := strings.ToLower(filepath.Clean(path))
		if previous, exists := paths[key]; exists {
			return contracts.GenerationResult{}, fmt.Errorf("special templates %d and %d resolve to the same output %s", previous, record.ID, path)
		}
		paths[key] = record.ID
		if strings.TrimSpace(record.Source) == "" {
			return contracts.GenerationResult{}, fmt.Errorf("special template %d has empty source_code", record.ID)
		}
		if _, err := template.New(record.Name).Parse(record.Source); err != nil {
			return contracts.GenerationResult{}, fmt.Errorf("parse special template %d: %w", record.ID, err)
		}
	}
	generated := 0
	for index, record := range records {
		if err := ctx.Err(); err != nil {
			return contracts.GenerationResult{}, err
		}
		contracts.ReportProgress(ctx, contracts.Progress{Stage: "生成专题页", Processed: index, Total: len(records), GeneratedFiles: generated})
		if _, err := s.generate(ctx, record); err != nil {
			return contracts.GenerationResult{}, err
		}
		generated++
	}
	contracts.ReportProgress(ctx, contracts.Progress{Stage: "专题页生成完成", Processed: len(records), Total: len(records), GeneratedFiles: generated})
	return contracts.GenerationResult{
		GeneratedAt: nowAt(s.Location), DurationSeconds: durationSeconds(started), GeneratedFiles: generated,
		GeneratedPages: generated, TotalItems: len(records), Output: s.outputRoot(ctx),
	}, nil
}

func (s TopicService) Generate(ctx context.Context, id int64) (contracts.TopicResult, error) {
	record, err := s.Store.LoadTemplateByID(ctx, id, "special")
	if err != nil {
		return contracts.TopicResult{}, errors.Join(contracts.ErrTopicNotFound, err)
	}
	return s.generate(ctx, record)
}

func (s TopicService) generate(ctx context.Context, record BoundTemplate) (contracts.TopicResult, error) {
	started := time.Now()
	parsed, err := template.New(record.Name).Parse(record.Source)
	if err != nil {
		return contracts.TopicResult{}, fmt.Errorf("parse special template %d: %w", record.ID, err)
	}
	path, err := s.outputPath(ctx, record.RoutePath)
	if err != nil {
		return contracts.TopicResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return contracts.TopicResult{}, fmt.Errorf("create special template directory: %w", err)
	}
	if err := s.ensureSafeDestination(s.outputRoot(ctx), path); err != nil {
		return contracts.TopicResult{}, err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".topic-*.tmp")
	if err != nil {
		return contracts.TopicResult{}, fmt.Errorf("create special template output: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	data := topicTemplateData{GeneratedAt: nowAt(s.Location).Format("2006-01-02 15:04:05"), Template: record}
	if err := parsed.Execute(temp, data); err != nil {
		_ = temp.Close()
		return contracts.TopicResult{}, fmt.Errorf("render special template %d: %w", record.ID, err)
	}
	if err := temp.Close(); err != nil {
		return contracts.TopicResult{}, fmt.Errorf("close special template output: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return contracts.TopicResult{}, fmt.Errorf("publish special template %d: %w", record.ID, err)
	}
	return contracts.TopicResult{
		GeneratedAt: nowAt(s.Location), DurationSeconds: durationSeconds(started), GeneratedFiles: 1,
		TemplateID: record.ID, RoutePath: record.RoutePath, Output: path,
	}, nil
}

func (s TopicService) Delete(ctx context.Context, id int64) (contracts.DeleteTopicResult, error) {
	record, err := s.Store.LoadTemplateByID(ctx, id, "special")
	if err != nil {
		return contracts.DeleteTopicResult{}, errors.Join(contracts.ErrTopicNotFound, err)
	}
	path, err := s.outputPath(ctx, record.RoutePath)
	if err != nil {
		return contracts.DeleteTopicResult{}, err
	}
	if err := s.ensureSafeDestination(s.outputRoot(ctx), path); err != nil {
		return contracts.DeleteTopicResult{}, err
	}
	deleted := false
	if err := os.Remove(path); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return contracts.DeleteTopicResult{}, fmt.Errorf("delete special template %d: %w", id, err)
		}
	} else {
		deleted = true
	}
	paths := []string(nil)
	if deleted {
		paths = []string{path}
	}
	return contracts.DeleteTopicResult{DeletedAt: nowAt(s.Location), TemplateID: id, Deleted: deleted, DeletedPaths: paths}, nil
}

func (s TopicService) outputRoot(ctx context.Context) string {
	if output := strings.TrimSpace(contracts.OptionsFrom(ctx).OutputPath); output != "" {
		return filepath.Clean(output)
	}
	return filepath.Clean(s.AllowedRoot)
}

func (s TopicService) outputPath(ctx context.Context, routePath string) (string, error) {
	root, err := filepath.Abs(s.outputRoot(ctx))
	if err != nil || root == filepath.VolumeName(root)+string(filepath.Separator) {
		return "", contracts.ErrInvalidOutputPath
	}
	allowed, err := filepath.Abs(filepath.Clean(s.AllowedRoot))
	if err != nil {
		return "", err
	}
	if err := requirePathWithin(allowed, root); err != nil {
		return "", fmt.Errorf("%w: output root is outside the configured root", contracts.ErrInvalidOutputPath)
	}
	rel, err := routeOutputFile(routePath)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := requirePathWithin(root, path); err != nil {
		return "", err
	}
	return path, nil
}

func routeOutputFile(routePath string) (string, error) {
	route := strings.TrimSpace(strings.ReplaceAll(routePath, "\\", "/"))
	if route == "" || route == "/" {
		return "index.html", nil
	}
	if strings.Contains(route, "://") || strings.ContainsRune(route, '\x00') {
		return "", fmt.Errorf("%w: invalid route_path %q", contracts.ErrInvalidOutputPath, routePath)
	}
	trailingSlash := strings.HasSuffix(route, "/")
	route = strings.Trim(route, "/")
	clean := filepath.Clean(filepath.FromSlash(route))
	if clean == "." || clean == ".." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: invalid route_path %q", contracts.ErrInvalidOutputPath, routePath)
	}
	if trailingSlash {
		return filepath.ToSlash(filepath.Join(clean, "index.html")), nil
	}
	if filepath.Ext(clean) == "" {
		clean += ".html"
	}
	return filepath.ToSlash(clean), nil
}

func (s TopicService) ensureSafeDestination(outputRoot, path string) error {
	root, err := filepath.Abs(filepath.Clean(outputRoot))
	if err != nil {
		return err
	}
	if err := requirePathWithin(root, path); err != nil {
		return err
	}
	realRoot, err := evalOrPlain(root)
	if err != nil {
		return fmt.Errorf("resolve allowed output root: %w", err)
	}
	ancestor := filepath.Dir(path)
	for {
		if _, statErr := os.Lstat(ancestor); statErr == nil {
			break
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return statErr
		}
		next := filepath.Dir(ancestor)
		if next == ancestor {
			return contracts.ErrInvalidOutputPath
		}
		ancestor = next
	}
	realAncestor, err := evalOrPlain(ancestor)
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}
	return requirePathWithin(realRoot, realAncestor)
}

func evalOrPlain(path string) (string, error) {
	real, err := filepath.EvalSymlinks(path)
	if err == nil {
		return real, nil
	}
	info, statErr := os.Lstat(path)
	if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
		return "", err
	}
	return filepath.Clean(path), nil
}

func requirePathWithin(root, path string) error {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return contracts.ErrInvalidOutputPath
	}
	return nil
}

func nowAt(location *time.Location) time.Time {
	if location == nil {
		return time.Now()
	}
	return time.Now().In(location)
}

func durationSeconds(started time.Time) float64 {
	return float64(time.Since(started).Milliseconds()) / 1000
}
