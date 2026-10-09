package generator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"portal-static/internal/adapters/camie/config"
	"portal-static/internal/adapters/camie/model"
	"portal-static/internal/contracts"
)

func NormalizePageName(name string) (string, bool) {
	switch strings.TrimSpace(name) {
	case "首页", "home", "index":
		return "home", true
	case "普通栏目", "list", "column":
		return "list", true
	case "普通详情", "article", "detail":
		return "article", true
	case "视频栏目", "videos":
		return "videos", true
	case "视频详情", "video":
		return "video", true
	case "搜索页", "search":
		return "search", true
	case "会员动态壳", "member":
		return "member", true
	default:
		return "", false
	}
}

func (g *Generator) ValidateOutputPath(raw string) error {
	raw = strings.TrimSpace(raw)
	if !filepath.IsAbs(raw) {
		return fmt.Errorf("%w: path must be absolute", contracts.ErrInvalidOutputPath)
	}
	allowedRoot := strings.TrimSpace(g.cfg.Site.AllowedOutputRoot)
	if allowedRoot == "" {
		allowedRoot = g.cfg.Site.DistRoot
	}
	allowed, err := filepath.Abs(allowedRoot)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(raw)
	if err != nil {
		return err
	}
	allowed, target = filepath.Clean(allowed), filepath.Clean(target)
	if filepath.Dir(target) == target {
		return fmt.Errorf("%w: path must not be a volume root", contracts.ErrInvalidOutputPath)
	}
	rel, err := filepath.Rel(allowed, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("%w: path must be the configured allowed_output_root or one of its descendants", contracts.ErrInvalidOutputPath)
	}
	return configOutput(g.cfg.Site.SourceRoot, target)
}

func configOutput(source, target string) error {
	if err := config.ValidateOutput(source, target); err != nil {
		return fmt.Errorf("%w: %v", contracts.ErrInvalidOutputPath, err)
	}
	return nil
}

func (g *Generator) Column(ctx context.Context, selector string, id int64) (model.Column, int, error) {
	c, err := g.load(ctx)
	if err != nil {
		return model.Column{}, 0, err
	}
	var matches []model.Column
	for _, column := range c.columns {
		if isPrivateColumnCode(column.Code) {
			continue
		}
		if (id > 0 && column.ID == id) || (id == 0 && (column.Code == selector || column.Name == selector)) {
			matches = append(matches, column)
		}
	}
	if len(matches) == 0 {
		return model.Column{}, 0, contracts.ErrColumnNotFound
	}
	if len(matches) > 1 {
		return model.Column{}, 0, contracts.ErrColumnNotUnique
	}
	return matches[0], len(c.groups[matches[0].ID]), nil
}

func (g *Generator) Article(ctx context.Context, id int64) (*Article, error) {
	c, err := g.load(ctx)
	if err != nil {
		return nil, err
	}
	article := c.byID[id]
	if article == nil {
		return nil, contracts.ErrArticleNotPublished
	}
	copy := *article
	return &copy, nil
}
