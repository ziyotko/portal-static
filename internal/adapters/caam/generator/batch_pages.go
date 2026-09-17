package generator

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

func (g *SiteGenerator) GeneratePages(ctx context.Context) (AllPagesResult, error) {
	target, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return AllPagesResult{}, err
	}
	if target != g {
		return target.GeneratePages(runCtx)
	}
	ctx = runCtx
	if g.home == nil {
		return AllPagesResult{}, errors.New("home page generation is unavailable")
	}
	started := time.Now()
	if err := g.prepareDist(); err != nil {
		return AllPagesResult{}, err
	}
	targetRoot := filepath.Clean(g.cfg.Site.DistRoot)
	parent := filepath.Dir(targetRoot)
	staging, err := os.MkdirTemp(parent, ".all-pages-*")
	if err != nil {
		return AllPagesResult{}, fmt.Errorf("create page staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	stageCfg := g.cfg
	stageCfg.Site.DistRoot = staging
	stageCfg.Site.Output = filepath.Join(staging, "index.html")
	pageCfg := stageCfg
	pageCfg.Site.OutputRoot = staging
	stagePages, err := NewPageGenerator(pageCfg, g.homeSource, g.logger)
	if err != nil {
		return AllPagesResult{}, err
	}
	stageHome := *g.home
	stageHome.cfg = stageCfg
	stage := NewSiteGenerator(stageCfg, g.homeSource, g.aboutSource, stagePages, &stageHome, g.logger)
	stage.skipScaffoldCopy = true
	stage.grayscale = g.grayscale
	stage.home.grayscale = g.grayscale

	names := []string{"home", "about", "work", "stats", "members", "party"}
	files := []string{"index.html", stageCfg.About.Output, stageCfg.WorkPage.Output, stageCfg.StatsPage.Output, stageCfg.MembersPage.Output, stageCfg.PartyPage.Output}
	pages := make([]StaticPageResult, 0, len(names))
	for index, name := range names {
		reportProgress(ctx, Progress{Stage: "生成主页面", Processed: index, Total: len(names), GeneratedFiles: index})
		g.logger.Info("生成主页面", "page", name)
		result, pageErr := stage.GeneratePage(ctx, name)
		if pageErr != nil {
			return AllPagesResult{}, wrapStaticPageError(name, pageErr)
		}
		pages = append(pages, result)
		reportProgress(ctx, Progress{Stage: "生成主页面", Processed: index + 1, Total: len(names), GeneratedFiles: index + 1})
	}
	if err := publishPageSet(staging, targetRoot, files); err != nil {
		return AllPagesResult{}, err
	}
	for index := range pages {
		pages[index].Output = filepath.Join(targetRoot, files[index])
	}
	return AllPagesResult{
		GeneratedAt:      time.Now().In(g.pages.location),
		DurationSeconds:  math.Round(time.Since(started).Seconds()*1000) / 1000,
		GeneratedFiles:   countStaticPageFiles(pages),
		GeneratedDetails: countStaticPageDetails(pages),
		GeneratedLists:   countStaticPageLists(pages),
		Generated:        len(pages), Pages: pages, Output: targetRoot, Gray: grayCode(g.grayscale),
	}, nil
}

func publishPageSet(staging, targetRoot string, files []string) error {
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return fmt.Errorf("create page output root: %w", err)
	}
	type previousFile struct {
		data   []byte
		exists bool
	}
	previous := make(map[string]previousFile, len(files))
	generated := make(map[string][]byte, len(files))
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(staging, name))
		if err != nil {
			return fmt.Errorf("validate staged page %s: %w", name, err)
		}
		if len(data) == 0 {
			return fmt.Errorf("validate staged page %s: file is empty", name)
		}
		generated[name] = data
		old, readErr := os.ReadFile(filepath.Join(targetRoot, name))
		switch {
		case readErr == nil:
			previous[name] = previousFile{data: old, exists: true}
		case errors.Is(readErr, os.ErrNotExist):
			previous[name] = previousFile{}
		default:
			return fmt.Errorf("read current page %s: %w", name, readErr)
		}
	}
	published := make([]string, 0, len(files))
	for _, name := range files {
		target := filepath.Join(targetRoot, name)
		if err := publishFile(target, generated[name]); err != nil {
			for index := len(published) - 1; index >= 0; index-- {
				publishedName := published[index]
				old := previous[publishedName]
				publishedTarget := filepath.Join(targetRoot, publishedName)
				if old.exists {
					_ = publishFile(publishedTarget, old.data)
				} else {
					_ = os.Remove(publishedTarget)
				}
			}
			return fmt.Errorf("publish page set at %s: %w", name, err)
		}
		published = append(published, name)
	}
	return nil
}
