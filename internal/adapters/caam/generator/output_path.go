package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"portal-static/internal/contracts"
)

type outputPathContextKey struct{}

var ErrInvalidOutputPath = contracts.ErrInvalidOutputPath

// WithOutputPath attaches a normalized per-request deployment root to ctx.
func WithOutputPath(ctx context.Context, outputPath string) context.Context {
	return context.WithValue(ctx, outputPathContextKey{}, filepath.Clean(strings.TrimSpace(outputPath)))
}

// OutputPath returns the per-request deployment root, if one was supplied.
func OutputPath(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	// An explicitly stored empty value suppresses the request-level option.
	// This is required while an operation delegates to a staging generator:
	// otherwise the nested generator sees the original requested deployment
	// path again and writes outside its staging directory.
	if value, ok := ctx.Value(outputPathContextKey{}).(string); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(contracts.OptionsFrom(ctx).OutputPath)
}

func withoutOutputPath(ctx context.Context) context.Context {
	return context.WithValue(ctx, outputPathContextKey{}, "")
}

func (g *SiteGenerator) ValidateOutputPath(outputPath string) error {
	target := filepath.Clean(strings.TrimSpace(outputPath))
	if target == "." || !filepath.IsAbs(target) {
		return fmt.Errorf("%w: path must be an absolute directory", ErrInvalidOutputPath)
	}
	if filepath.Dir(target) == target {
		return fmt.Errorf("%w: path cannot be a filesystem root", ErrInvalidOutputPath)
	}
	source := filepath.Clean(g.cfg.Site.OutputRoot)
	if pathsOverlap(source, target) {
		return fmt.Errorf("%w: path cannot overlap the static source directory", ErrInvalidOutputPath)
	}
	if !pathContains(filepath.Clean(g.cfg.Site.DistRoot), target) {
		return fmt.Errorf("%w: path must be the configured dist_root or one of its descendants", ErrInvalidOutputPath)
	}
	if !resolvedPathContains(filepath.Clean(g.cfg.Site.DistRoot), target) {
		return fmt.Errorf("%w: path escapes the configured dist_root through a symbolic link", ErrInvalidOutputPath)
	}
	return nil
}

func resolvedPathContains(parent, child string) bool {
	realParent, ok := resolveProspectiveOutputPath(parent)
	if !ok {
		return false
	}
	realChild, ok := resolveProspectiveOutputPath(child)
	return ok && pathContains(realParent, realChild)
}

func resolveProspectiveOutputPath(path string) (string, bool) {
	path = filepath.Clean(path)
	ancestor := path
	for {
		if _, err := os.Lstat(ancestor); err == nil {
			break
		}
		next := filepath.Dir(ancestor)
		if next == ancestor {
			return "", false
		}
		ancestor = next
	}
	realAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		info, statErr := os.Lstat(ancestor)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", false
		}
		realAncestor = ancestor
	}
	rel, err := filepath.Rel(ancestor, path)
	if err != nil {
		return "", false
	}
	return filepath.Join(realAncestor, rel), true
}

func pathsOverlap(first, second string) bool {
	return pathContains(first, second) || pathContains(second, first)
}

func pathContains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if relative == "." {
		return true
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (g *SiteGenerator) generatorForOutputPath(ctx context.Context) (*SiteGenerator, context.Context, error) {
	requested := OutputPath(ctx)
	requestedGrayscale, hasGrayscale := grayscaleOption(ctx)
	targetGrayscale := g.grayscale
	if hasGrayscale {
		targetGrayscale = requestedGrayscale
	}
	outputChanged := requested != "" && filepath.Clean(requested) != filepath.Clean(g.cfg.Site.DistRoot)
	grayscaleChanged := targetGrayscale != g.grayscale
	cleanCtx := withoutGrayscale(withoutOutputPath(ctx))
	if !outputChanged && !grayscaleChanged {
		return g, cleanCtx, nil
	}
	if outputChanged {
		if err := g.ValidateOutputPath(requested); err != nil {
			return nil, ctx, err
		}
	}
	clone := *g
	clone.grayscale = targetGrayscale
	target := clone.cfg.Site.DistRoot
	if outputChanged {
		target = filepath.Clean(requested)
		clone.cfg.Site.DistRoot = target
		clone.cfg.Site.Output = filepath.Join(target, "index.html")
	}
	if g.pages != nil {
		pages := *g.pages
		if outputChanged {
			pages.cfg.Site.DistRoot = target
			pages.cfg.Site.OutputRoot = target
			pages.cfg.Site.Output = filepath.Join(target, "index.html")
		}
		clone.pages = &pages
	}
	if g.home != nil {
		home := *g.home
		home.grayscale = targetGrayscale
		if outputChanged {
			home.cfg.Site.DistRoot = target
			home.cfg.Site.OutputRoot = target
			home.cfg.Site.Output = filepath.Join(target, "index.html")
		}
		clone.home = &home
	}
	return &clone, cleanCtx, nil
}
