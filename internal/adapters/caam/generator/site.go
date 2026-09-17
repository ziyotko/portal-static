package generator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"log/slog"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
)

type AboutSource interface {
	FetchPublishedByColumnName(context.Context, string) (model.Column, []model.Article, error)
	FetchPublishedByColumnNameLimit(context.Context, string, []int, int) (model.Column, []model.Article, error)
	FetchPublishedByColumnID(context.Context, int64) (model.Column, []model.Article, error)
}

type PreparedColumn struct {
	Key      string
	Title    string
	Column   model.Column
	Articles []model.Article
}

type SiteArticlesResult struct {
	GeneratedAt      time.Time `json:"generated_at"`
	DurationSeconds  float64   `json:"duration_seconds"`
	GeneratedFiles   int       `json:"generated_files"`
	GeneratedDetails int       `json:"generated_details"`
	GeneratedLists   int       `json:"generated_lists"`
	Generated        int       `json:"generated"`
	SkippedExternal  int       `json:"skipped_external"`
	SkippedData      int       `json:"skipped_data"`
	SkippedInvalid   int       `json:"skipped_invalid"`
	Output           string    `json:"output"`
}

type StaticPageResult struct {
	GeneratedAt      time.Time `json:"generated_at"`
	DurationSeconds  float64   `json:"duration_seconds"`
	GeneratedFiles   int       `json:"generated_files"`
	Page             string    `json:"page"`
	TotalItems       int       `json:"total_items"`
	GeneratedDetails int       `json:"generated_details"`
	GeneratedLists   int       `json:"generated_lists"`
	Output           string    `json:"output"`
	Gray             string    `json:"gray"`
}

type SiteGenerationResult struct {
	GeneratedAt      time.Time          `json:"generated_at"`
	DurationSeconds  float64            `json:"duration_seconds"`
	GeneratedFiles   int                `json:"generated_files"`
	GeneratedDetails int                `json:"generated_details"`
	GeneratedLists   int                `json:"generated_lists"`
	GeneratedColumns int                `json:"generated_columns"`
	TotalListPages   int                `json:"total_list_pages"`
	Pages            []StaticPageResult `json:"pages"`
	Output           string             `json:"output"`
	Gray             string             `json:"gray"`
}

func (r SiteGenerationResult) GeneratedFileCount() int { return r.GeneratedFiles }

type AllPagesResult struct {
	GeneratedAt      time.Time          `json:"generated_at"`
	DurationSeconds  float64            `json:"duration_seconds"`
	GeneratedFiles   int                `json:"generated_files"`
	GeneratedDetails int                `json:"generated_details"`
	GeneratedLists   int                `json:"generated_lists"`
	Generated        int                `json:"generated_pages"`
	Pages            []StaticPageResult `json:"pages"`
	Output           string             `json:"output"`
	Gray             string             `json:"gray"`
}

func (r AllPagesResult) GeneratedFileCount() int { return r.GeneratedFiles }

type AboutMemberView struct {
	Title string
	Href  string
}

type LeaderView struct {
	ArticleView
	Name string
	Role string
}

type AboutPageData struct {
	GeneratedAt      string
	Intro            template.HTML
	Leaders          []LeaderView
	Charter          []ArticleView
	Organization     []ArticleView
	OrganizationCols [][]ArticleView
	Responsibilities []ArticleView
	Honors           []ArticleView
	HonorCols        [][]ArticleView
	Members          []AboutMemberView
	FooterLinks      []LinkGroupView
}

type SiteGenerator struct {
	cfg              config.Config
	homeSource       PageSource
	aboutSource      AboutSource
	pages            *PageGenerator
	home             *Generator
	logger           *slog.Logger
	skipScaffoldCopy bool
	grayscale        bool
}

func NewSiteGenerator(cfg config.Config, homeSource PageSource, aboutSource AboutSource, pages *PageGenerator, home *Generator, logger *slog.Logger) *SiteGenerator {
	if logger == nil {
		logger = slog.Default()
	}
	return &SiteGenerator{cfg: cfg, homeSource: homeSource, aboutSource: aboutSource, pages: pages, home: home, logger: logger}
}

func (g *SiteGenerator) prepareDist() error {
	if g.skipScaffoldCopy {
		return nil
	}
	return ensureStaticScaffold(g.cfg.Site.OutputRoot, g.cfg.Site.DistRoot, g.managedPages())
}

func (g *SiteGenerator) GenerateAllLists(ctx context.Context) (AllListsResult, error) {
	target, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return AllListsResult{}, err
	}
	if target != g {
		return target.GenerateAllLists(runCtx)
	}
	ctx = runCtx
	if err := g.prepareDist(); err != nil {
		return AllListsResult{}, err
	}
	return g.pages.GenerateAllLists(ctx)
}

func (g *SiteGenerator) GenerateAllArticles(ctx context.Context) (AllArticlesResult, error) {
	target, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return AllArticlesResult{}, err
	}
	if target != g {
		return target.GenerateAllArticles(runCtx)
	}
	ctx = runCtx
	if err := g.prepareDist(); err != nil {
		return AllArticlesResult{}, err
	}
	return g.pages.GenerateAllArticles(ctx)
}

func (g *SiteGenerator) GenerateList(ctx context.Context, columnID int64) (ListResult, error) {
	started := time.Now()
	target, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return ListResult{}, err
	}
	if target != g {
		result, targetErr := target.GenerateList(runCtx, columnID)
		if targetErr == nil {
			result.DurationSeconds = elapsedSeconds(started)
			result.GeneratedFiles = result.TotalPages
			result.GeneratedLists = result.TotalPages
		}
		return result, targetErr
	}
	ctx = runCtx
	if err := g.prepareDist(); err != nil {
		return ListResult{}, err
	}
	result, err := g.pages.GenerateList(ctx, columnID)
	if err == nil {
		result.DurationSeconds = elapsedSeconds(started)
		result.GeneratedFiles = result.TotalPages
		result.GeneratedLists = result.TotalPages
	}
	return result, err
}

func (g *SiteGenerator) GenerateListByName(ctx context.Context, columnName string) (ListResult, error) {
	started := time.Now()
	target, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return ListResult{}, err
	}
	if target != g {
		result, targetErr := target.GenerateListByName(runCtx, columnName)
		if targetErr == nil {
			result.DurationSeconds = elapsedSeconds(started)
			result.GeneratedFiles = result.TotalPages
			result.GeneratedLists = result.TotalPages
		}
		return result, targetErr
	}
	ctx = runCtx
	if err := g.prepareDist(); err != nil {
		return ListResult{}, err
	}
	result, err := g.pages.GenerateListByName(ctx, columnName)
	if err == nil {
		result.DurationSeconds = elapsedSeconds(started)
		result.GeneratedFiles = result.TotalPages
		result.GeneratedLists = result.TotalPages
	}
	return result, err
}

func (g *SiteGenerator) GenerateArticle(ctx context.Context, articleID int64) (ArticleResult, error) {
	started := time.Now()
	target, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return ArticleResult{}, err
	}
	if target != g {
		result, targetErr := target.GenerateArticle(runCtx, articleID)
		if targetErr == nil {
			result.DurationSeconds = elapsedSeconds(started)
			result.GeneratedFiles = 1
			result.GeneratedDetails = 1
		}
		return result, targetErr
	}
	ctx = runCtx
	if err := g.prepareDist(); err != nil {
		return ArticleResult{}, err
	}
	result, err := g.pages.GenerateArticle(ctx, articleID)
	if err == nil {
		result.DurationSeconds = elapsedSeconds(started)
		result.GeneratedFiles = 1
		result.GeneratedDetails = 1
	}
	return result, err
}

func (g *SiteGenerator) DeleteArticle(ctx context.Context, articleID int64) (DeleteArticleResult, error) {
	target, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return DeleteArticleResult{}, err
	}
	if target != g {
		return target.DeleteArticle(runCtx, articleID)
	}
	ctx = runCtx
	return g.pages.DeleteArticle(ctx, articleID)
}

func (g *SiteGenerator) generatePagesLegacy(ctx context.Context) (AllPagesResult, error) {
	started := time.Now()
	if err := g.prepareDist(); err != nil {
		return AllPagesResult{}, err
	}
	pageGenerator := *g
	pageGenerator.skipScaffoldCopy = true
	names := []string{"home", "about", "work", "stats", "members", "party"}
	pages := make([]StaticPageResult, 0, len(names))
	for index, name := range names {
		reportProgress(ctx, Progress{Stage: "生成主页面", Processed: index, Total: len(names)})
		g.logger.Info("生成主页面", "page", name)
		result, err := pageGenerator.GeneratePage(ctx, name)
		if err != nil {
			return AllPagesResult{}, wrapStaticPageError(name, err)
		}
		pages = append(pages, result)
		reportProgress(ctx, Progress{Stage: "生成主页面", Processed: index + 1, Total: len(names), GeneratedFiles: index + 1})
	}
	return AllPagesResult{
		GeneratedAt: time.Now().In(g.pages.location), DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000,
		GeneratedFiles: countStaticPageFiles(pages), GeneratedDetails: countStaticPageDetails(pages),
		GeneratedLists: countStaticPageLists(pages), Generated: len(pages), Pages: pages,
		Output: g.cfg.Site.DistRoot, Gray: grayCode(g.grayscale),
	}, nil
}

func (g *SiteGenerator) GenerateSiteArticles(ctx context.Context) (SiteArticlesResult, error) {
	target, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return SiteArticlesResult{}, err
	}
	if target != g {
		return target.GenerateSiteArticles(runCtx)
	}
	ctx = runCtx
	if err := g.prepareDist(); err != nil {
		return SiteArticlesResult{}, err
	}
	g.logger.Info("加载静态页栏目数据", "page", "home")
	home, err := g.loadHomeColumns(ctx)
	if err != nil {
		return SiteArticlesResult{}, err
	}
	g.logger.Info("加载静态页栏目数据", "page", "about")
	about, err := g.loadAboutColumns(ctx)
	if err != nil {
		return SiteArticlesResult{}, err
	}
	g.logger.Info("加载静态页栏目数据", "page", "work")
	work, err := g.loadWorkColumns(ctx)
	if err != nil {
		return SiteArticlesResult{}, err
	}
	g.logger.Info("加载静态页栏目数据", "page", "stats")
	stats, err := g.loadStatsColumns(ctx)
	if err != nil {
		return SiteArticlesResult{}, err
	}
	columns := append(home, about...)
	columns = append(columns, work...)
	columns = append(columns, stats...)
	g.logger.Info("加载静态页栏目数据", "page", "members")
	members, err := g.loadMembersColumns(ctx)
	if err != nil {
		return SiteArticlesResult{}, err
	}
	g.logger.Info("加载静态页栏目数据", "page", "party")
	party, err := g.loadPartyColumns(ctx)
	if err != nil {
		return SiteArticlesResult{}, err
	}
	columns = append(columns, members...)
	columns = append(columns, party...)
	return g.pages.generatePreparedArticles(ctx, columns, ".site-articles.lock")
}

// GenerateSite builds a complete deployment in a sibling staging directory
// and only replaces dist_root after every detail, list and main page succeeds.
func (g *SiteGenerator) GenerateSite(ctx context.Context) (SiteGenerationResult, error) {
	requested, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return SiteGenerationResult{}, err
	}
	if requested != g {
		return requested.GenerateSite(runCtx)
	}
	ctx = runCtx
	if g.home == nil {
		return SiteGenerationResult{}, errors.New("home page generation is unavailable")
	}
	started := time.Now()
	target := filepath.Clean(g.cfg.Site.DistRoot)
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return SiteGenerationResult{}, fmt.Errorf("create site output parent: %w", err)
	}
	lock, err := acquireFileLock(target+".lock", g.pages.staleAfter)
	if err != nil {
		return SiteGenerationResult{}, err
	}
	defer lock.release()

	staging, err := os.MkdirTemp(parent, ".caam-site-*")
	if err != nil {
		return SiteGenerationResult{}, fmt.Errorf("create site staging directory: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(staging)
		}
	}()

	stageCfg := g.cfg
	stageCfg.Site.DistRoot = staging
	stageCfg.Site.Output = filepath.Join(staging, "index.html")
	pageCfg := stageCfg
	pageCfg.Site.OutputRoot = staging
	stagePages, err := NewPageGenerator(pageCfg, g.homeSource, g.logger)
	if err != nil {
		return SiteGenerationResult{}, err
	}
	stageHome := *g.home
	stageHome.cfg = stageCfg
	stage := NewSiteGenerator(stageCfg, g.homeSource, g.aboutSource, stagePages, &stageHome, g.logger)
	stage.grayscale = g.grayscale
	stage.home.grayscale = g.grayscale
	g.logger.Info("整站静态化开始", "output", target)
	g.logger.Info("整站静态化阶段", "stage", "复制静态资源")
	reportProgress(ctx, Progress{Stage: "复制静态资源"})
	if err := stage.prepareDist(); err != nil {
		return SiteGenerationResult{}, fmt.Errorf("prepare site scaffold: %w", err)
	}
	stage.skipScaffoldCopy = true

	g.logger.Info("整站静态化阶段", "stage", "生成文章详情")
	reportProgress(ctx, Progress{Stage: "读取内容快照"})
	snapshot, err := stage.pages.loadContentSnapshot(ctx)
	if err != nil {
		return SiteGenerationResult{}, fmt.Errorf("load site content snapshot: %w", err)
	}
	details, err := stage.pages.generateAllArticlesWithSnapshot(ctx, snapshot)
	if err != nil {
		return SiteGenerationResult{}, fmt.Errorf("generate site details: %w", err)
	}
	g.logger.Info("整站静态化阶段完成", "stage", "生成文章详情", "generated", details.Generated)
	g.logger.Info("整站静态化阶段", "stage", "生成栏目列表")
	lists, err := stage.pages.generateAllListsWithSnapshot(ctx, snapshot)
	if err != nil {
		return SiteGenerationResult{}, fmt.Errorf("generate site lists: %w", err)
	}
	g.logger.Info("整站静态化阶段完成", "stage", "生成栏目列表", "columns", lists.Generated, "pages", lists.TotalPages)
	pageNames := []string{"home", "about", "work", "stats", "members", "party"}
	pages := make([]StaticPageResult, 0, len(pageNames))
	for index, name := range pageNames {
		reportProgress(ctx, Progress{Stage: "生成主页面", Processed: index, Total: len(pageNames)})
		g.logger.Info("整站静态化阶段", "stage", "生成主页面", "page", name)
		page, pageErr := stage.GeneratePage(ctx, name)
		if pageErr != nil {
			return SiteGenerationResult{}, wrapStaticPageError(name, pageErr)
		}
		pages = append(pages, page)
		reportProgress(ctx, Progress{Stage: "生成主页面", Processed: index + 1, Total: len(pageNames), GeneratedFiles: index + 1})
	}
	for _, required := range []string{"index.html", stageCfg.About.Output, stageCfg.WorkPage.Output, stageCfg.StatsPage.Output, stageCfg.MembersPage.Output, stageCfg.PartyPage.Output} {
		info, statErr := os.Stat(filepath.Join(staging, required))
		if statErr != nil {
			return SiteGenerationResult{}, fmt.Errorf("validate generated site page %s: %w", required, statErr)
		}
		if info.IsDir() || info.Size() == 0 {
			return SiteGenerationResult{}, fmt.Errorf("validate generated site page %s: output is empty", required)
		}
	}
	if g.grayscale {
		if err := applyGrayscaleToHTMLTree(staging); err != nil {
			return SiteGenerationResult{}, fmt.Errorf("apply site grayscale: %w", err)
		}
	}
	mainOutputs := []string{"index.html", stageCfg.About.Output, stageCfg.WorkPage.Output, stageCfg.StatsPage.Output, stageCfg.MembersPage.Output, stageCfg.PartyPage.Output}
	reportProgress(ctx, Progress{Stage: "校验内部链接"})
	validatedLinks, err := validateSiteInternalLinks(staging, mainOutputs)
	if err != nil {
		return SiteGenerationResult{}, fmt.Errorf("validate generated site links: %w", err)
	}
	g.logger.Info("整站静态化阶段完成", "stage", "校验内部链接", "targets", validatedLinks)
	reportProgress(ctx, Progress{Stage: "发布整站", Processed: 0, Total: 1})
	if err := replaceDirectory(staging, target); err != nil {
		return SiteGenerationResult{}, fmt.Errorf("publish generated site: %w", err)
	}
	generatedFiles := details.Generated + lists.TotalPages + len(pages)
	reportProgress(ctx, Progress{Stage: "发布整站", Processed: 1, Total: 1, GeneratedFiles: generatedFiles})
	cleanup = false

	for index := range pages {
		pages[index].Output = filepath.Join(target, filepath.Base(pages[index].Output))
	}
	result := SiteGenerationResult{
		GeneratedAt: time.Now().In(g.pages.location), DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000,
		GeneratedFiles:   generatedFiles,
		GeneratedDetails: details.Generated, GeneratedLists: lists.TotalPages,
		GeneratedColumns: lists.Generated, TotalListPages: lists.TotalPages,
		Pages: pages, Output: target, Gray: grayCode(g.grayscale),
	}
	g.logger.Info("整站静态化生成成功", "details", result.GeneratedDetails, "columns", result.GeneratedColumns, "pages", len(result.Pages), "generated_files", generatedFiles, "output", target)
	return result, nil
}

func (g *SiteGenerator) GeneratePage(ctx context.Context, name string) (StaticPageResult, error) {
	started := time.Now()
	normalizedName, ok := NormalizeStaticPageName(name)
	if !ok {
		return StaticPageResult{}, fmt.Errorf("不支持的静态页面：“%s”", name)
	}
	target, runCtx, err := g.generatorForOutputPath(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	if target != g {
		result, targetErr := target.GeneratePage(runCtx, normalizedName)
		if targetErr == nil {
			result.DurationSeconds = elapsedSeconds(started)
			result.GeneratedFiles = 1 + result.GeneratedDetails + result.GeneratedLists
		}
		return result, targetErr
	}
	ctx = runCtx
	var result StaticPageResult
	switch normalizedName {
	case "about":
		result, err = g.generateAbout(ctx)
	case "home":
		result, err = g.generateHome(ctx)
	case "work":
		result, err = g.generateWork(ctx)
	case "stats":
		result, err = g.generateStats(ctx)
	case "members":
		result, err = g.generateMembers(ctx)
	case "party":
		result, err = g.generateParty(ctx)
	default:
		return StaticPageResult{}, fmt.Errorf("不支持的静态页面：“%s”", name)
	}
	if err == nil {
		result.DurationSeconds = elapsedSeconds(started)
		result.GeneratedFiles = 1 + result.GeneratedDetails + result.GeneratedLists
	}
	return result, err
}

func elapsedSeconds(started time.Time) float64 {
	seconds := time.Since(started).Seconds()
	rounded := math.Round(seconds*1000) / 1000
	if rounded < 0.001 {
		return 0.001
	}
	return rounded
}

func countStaticPageFiles(pages []StaticPageResult) int {
	total := 0
	for _, page := range pages {
		total += page.GeneratedFiles
	}
	return total
}

func countStaticPageDetails(pages []StaticPageResult) int {
	total := 0
	for _, page := range pages {
		total += page.GeneratedDetails
	}
	return total
}

func countStaticPageLists(pages []StaticPageResult) int {
	total := 0
	for _, page := range pages {
		total += page.GeneratedLists
	}
	return total
}

// NormalizeStaticPageName converts the public Chinese page name to the
// generator's stable internal code. English codes remain accepted for
// backwards compatibility with existing callers.
func NormalizeStaticPageName(name string) (string, bool) {
	switch strings.TrimSpace(name) {
	case "首页", "home":
		return "home", true
	case "协会概况", "about":
		return "about", true
	case "协会工作", "work":
		return "work", true
	case "统计数据", "stats":
		return "stats", true
	case "会员专区", "members":
		return "members", true
	case "党建专区", "party":
		return "party", true
	default:
		return "", false
	}
}

func wrapStaticPageError(name string, err error) error {
	return fmt.Errorf("生成%s失败：%w", staticPageDisplayName(name), err)
}

func staticPageDisplayName(name string) string {
	normalizedName, ok := NormalizeStaticPageName(name)
	if !ok {
		return fmt.Sprintf("静态页面“%s”", name)
	}
	switch normalizedName {
	case "home":
		return "首页"
	case "about":
		return "协会概况页"
	case "work":
		return "协会工作页"
	case "stats":
		return "统计数据页"
	case "members":
		return "会员专区页"
	case "party":
		return "党建专区页"
	}
	return fmt.Sprintf("静态页面“%s”", name)
}

func (g *SiteGenerator) managedPages() map[string]struct{} {
	pages := map[string]struct{}{
		g.cfg.About.Output:       {},
		g.cfg.WorkPage.Output:    {},
		g.cfg.StatsPage.Output:   {},
		g.cfg.MembersPage.Output: {},
		g.cfg.PartyPage.Output:   {},
		"index.generated.html":   {},
	}
	if g.home != nil {
		pages[filepath.Base(g.home.cfg.Site.Output)] = struct{}{}
	}
	return pages
}

func (g *SiteGenerator) loadHomeColumns(ctx context.Context) ([]PreparedColumn, error) {
	seen := make(map[int64]struct{})
	result := make([]PreparedColumn, 0)
	for _, slot := range g.cfg.ContentSlots() {
		g.logger.Info("加载首页栏目", "key", slot.Key, "column", slot.Name, "limit", slot.Limit)
		articles, err := g.homeSource.FetchByColumn(ctx, slot)
		if errors.Is(err, ErrColumnNotFound) {
			g.logger.Warn("首页栏目不存在，使用空内容", "key", slot.Key, "column", slot.Name)
			result = append(result, PreparedColumn{
				Key: slot.Key, Title: slot.Title,
				Column: model.Column{Name: slot.Name}, Articles: nil,
			})
			continue
		} else if err != nil {
			return nil, err
		}
		var columnID int64
		if len(articles) > 0 {
			columnID = articles[0].ColumnID
		}
		if columnID <= 0 {
			resolvedID, err := g.homeSource.ResolveColumnID(ctx, slot)
			if errors.Is(err, ErrColumnNotFound) {
				g.logger.Warn("首页栏目不存在，使用空内容", "key", slot.Key, "column", slot.Name)
				result = append(result, PreparedColumn{
					Key: slot.Key, Title: slot.Title,
					Column: model.Column{Name: slot.Name}, Articles: nil,
				})
				continue
			} else if err != nil {
				return nil, err
			}
			columnID = resolvedID
		}
		if _, exists := seen[columnID]; exists {
			continue
		}
		seen[columnID] = struct{}{}
		result = append(result, PreparedColumn{
			Key: slot.Key, Title: slot.Title,
			Column: model.Column{ID: columnID, Name: slot.Name}, Articles: articles,
		})
	}
	return result, nil
}

func (g *SiteGenerator) aboutSpecs() []struct {
	key, title, name string
	member           bool
} {
	a := g.cfg.About
	return []struct {
		key, title, name string
		member           bool
	}{
		{"intro", "协会简介", a.Intro, false},
		{"leadership", "领导团队", a.Leadership, false},
		{"charter", "协会章程", a.Charter, false},
		{"organization", "组织机构", a.Organization, false},
		{"responsibilities", "主要职责", a.Responsibilities, false},
		{"honors", "协会荣誉", a.Honors, false},
		{"rotating-president", "轮值会长单位", a.RotatingPresident, true},
		{"vice-president", "副会长单位", a.VicePresident, true},
		{"executive-director", "常务理事单位", a.ExecutiveDirector, true},
		{"director", "理事单位", a.Director, true},
		{"member-delegate", "会员代表单位", a.MemberDelegate, true},
		{"regular-member", "普通会员单位", a.RegularMember, true},
	}
}

func (g *SiteGenerator) loadAboutColumns(ctx context.Context) ([]PreparedColumn, error) {
	specs := g.aboutSpecs()
	result := make([]PreparedColumn, 0, len(specs))
	for index, spec := range specs {
		column, articles, err := g.aboutSource.FetchPublishedByColumnName(ctx, spec.name)
		if errors.Is(err, ErrColumnNotFound) {
			g.logger.Warn("协会概况栏目不存在，使用空内容", "key", spec.key, "column", spec.name)
			column = model.Column{Name: spec.name}
			articles = nil
		} else if err != nil {
			return nil, err
		}
		if len(articles) == 0 {
			g.logger.Warn("协会概况栏目暂无内容", "key", spec.key, "column", spec.name)
		}
		result = append(result, PreparedColumn{Key: spec.key, Title: spec.title, Column: column, Articles: articles})
		reportProgress(ctx, Progress{Stage: "读取协会概况栏目", Processed: index + 1, Total: len(specs), CurrentColumnID: column.ID})
	}
	return result, nil
}

func (g *SiteGenerator) generateAbout(ctx context.Context) (StaticPageResult, error) {
	started := time.Now()
	// Do not copy the hand-maintained about.html over the last generated page.
	// Database, detail, list or template failures must leave that page intact.
	if err := g.prepareDist(); err != nil {
		return StaticPageResult{}, err
	}
	columns, err := g.loadAboutColumns(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	footerLinks, err := g.pages.fetchFooterLinks(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	byKey := make(map[string]PreparedColumn, len(columns))
	totalItems := 0
	for _, column := range columns {
		byKey[column.Key] = column
		totalItems += len(column.Articles)
	}
	output, err := g.renderAbout(byKey, footerLinks)
	if err != nil {
		return StaticPageResult{}, err
	}
	generatedAt := time.Now().In(g.pages.location)
	result := StaticPageResult{
		GeneratedAt: generatedAt, DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000,
		Page: "about", TotalItems: totalItems, Output: output, Gray: grayCode(g.grayscale),
	}
	g.logger.Info("协会概况静态页生成成功", "items", totalItems, "output", output)
	return result, nil
}

func (g *SiteGenerator) generateHome(ctx context.Context) (StaticPageResult, error) {
	if g.home == nil {
		return StaticPageResult{}, errors.New("home page generation is unavailable")
	}
	started := time.Now()
	if err := g.prepareDist(); err != nil {
		return StaticPageResult{}, err
	}
	home, err := g.home.Generate(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	legacyOutput := filepath.Join(g.cfg.Site.DistRoot, "index.generated.html")
	if filepath.Clean(home.Output) != filepath.Clean(legacyOutput) {
		if err := os.Remove(legacyOutput); err != nil && !errors.Is(err, os.ErrNotExist) {
			g.logger.Warn("清理旧首页文件失败", "path", legacyOutput, "error", err)
		}
	}
	result := StaticPageResult{
		GeneratedAt:     home.GeneratedAt,
		DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000,
		Page:            "home",
		TotalItems:      home.TotalItems,
		Output:          home.Output,
		Gray:            grayCode(g.grayscale),
	}
	g.logger.Info("首页静态页生成成功", "items", result.TotalItems, "details", result.GeneratedDetails, "output", result.Output)
	return result, nil
}

func (g *SiteGenerator) renderAbout(columns map[string]PreparedColumn, footerLinks []LinkGroupView) (string, error) {
	output := filepath.Join(g.cfg.Site.DistRoot, g.cfg.About.Output)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return "", fmt.Errorf("create about output directory: %w", err)
	}
	lock, err := acquireFileLock(output+".lock", g.pages.staleAfter)
	if err != nil {
		return "", err
	}
	defer lock.release()

	intro := template.HTML("<p>暂无内容</p>")
	if articles := columns["intro"].Articles; len(articles) > 0 {
		if strings.TrimSpace(articles[0].Content) != "" {
			intro = g.pages.sanitizeContent(articles[0].Content)
		} else if strings.TrimSpace(articles[0].Summary) != "" {
			intro = template.HTML("<p>" + html.EscapeString(strings.TrimSpace(articles[0].Summary)) + "</p>")
		}
	}
	leaders := g.leaderViews(columns["leadership"])
	members := make([]AboutMemberView, 0, 6)
	for _, spec := range g.aboutSpecs() {
		if !spec.member {
			continue
		}
		column := columns[spec.key]
		members = append(members, AboutMemberView{Title: spec.title, Href: columnListHref(column)})
	}
	organization := g.aboutViews(columns["organization"], "organization")
	honors := g.aboutViews(columns["honors"], "honors")
	data := AboutPageData{
		GeneratedAt: time.Now().In(g.pages.location).Format(time.RFC3339), Intro: intro,
		Leaders:          leaders,
		Charter:          g.aboutViews(columns["charter"], "charter"),
		Organization:     organization,
		OrganizationCols: splitAboutColumns(organization),
		Responsibilities: g.aboutViews(columns["responsibilities"], "responsibilities"),
		Honors:           honors,
		HonorCols:        splitAboutColumns(honors), Members: members, FooterLinks: footerLinks,
	}
	tpl, err := template.ParseFiles(g.cfg.About.Template)
	if err != nil {
		return "", fmt.Errorf("parse about template: %w", err)
	}
	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("render about page: %w", err)
	}
	renderedPage, err := applyGrayscale(rendered.Bytes(), g.grayscale)
	if err != nil {
		return "", err
	}
	rendered.Reset()
	_, _ = rendered.Write(renderedPage)
	if err := validateInnerPage(rendered.Bytes(), "class=\"about-intro\""); err != nil {
		return "", fmt.Errorf("validate about page: %w", err)
	}
	if bytes.Contains(rendered.Bytes(), []byte("href=\"#\"")) {
		return "", errors.New("rendered about page contains placeholder links")
	}
	if err := publishFile(output, rendered.Bytes()); err != nil {
		return "", err
	}
	return output, nil
}

func splitAboutColumns(items []ArticleView) [][]ArticleView {
	if len(items) == 0 {
		return nil
	}
	middle := (len(items) + 1) / 2
	columns := [][]ArticleView{items[:middle]}
	if middle < len(items) {
		columns = append(columns, items[middle:])
	}
	return columns
}

func (g *SiteGenerator) aboutViews(column PreparedColumn, section string) []ArticleView {
	builder := newViewBuilderForConfig(g.cfg, g.pages.location)
	views := make([]ArticleView, 0, len(column.Articles))
	for index, article := range column.Articles {
		view := builder.article(article, index, "")
		if href, external, ok := safeHref(strings.TrimSpace(article.URL)); ok && external {
			view.Href, view.Clickable, view.ExternalLink = href, true, true
		} else if href, ok := articleArchiveHref(article, g.pages.location); ok {
			query := url.Values{}
			query.Set("from", "about")
			query.Set("section", section)
			view.Href, view.Clickable = href+"?"+query.Encode(), true
		} else {
			g.logger.Warn("协会概况文章缺少有效详情地址", "column", column.Column.Name, "article_id", article.ID)
			continue
		}
		views = append(views, view)
	}
	return views
}

var leaderRoles = []string{
	"监事会监事长、专务副秘书长",
	"常务副会长兼秘书长",
	"总工程师、专务副秘书长",
	"总工程师、副秘书长",
	"党委副书记兼秘书长",
	"专务副秘书长",
	"秘书长助理",
	"副总工程师",
	"副秘书长",
}

func (g *SiteGenerator) leaderViews(column PreparedColumn) []LeaderView {
	articles := g.aboutViews(column, "leadership")
	leaders := make([]LeaderView, 0, len(articles))
	for _, article := range articles {
		name, role := splitLeaderTitle(article.Title, article.Summary)
		leaders = append(leaders, LeaderView{
			ArticleView: article,
			Name:        name,
			Role:        role,
		})
	}
	return leaders
}

func splitLeaderTitle(title, summary string) (string, string) {
	name := strings.TrimSpace(title)
	name = strings.TrimSpace(strings.TrimSuffix(name, "简历"))
	name = strings.TrimSpace(strings.TrimPrefix(name, "中国汽车工业协会"))
	role := strings.TrimSpace(summary)
	if role != "" {
		return name, role
	}
	for _, candidate := range leaderRoles {
		if strings.HasPrefix(name, candidate) {
			return strings.TrimSpace(strings.TrimPrefix(name, candidate)), candidate
		}
	}
	return name, ""
}

func (g *PageGenerator) generatePreparedArticles(ctx context.Context, columns []PreparedColumn, lockName string) (SiteArticlesResult, error) {
	started := time.Now()
	articleRoot := filepath.Join(g.cfg.Site.OutputRoot, "article")
	lockRoot := filepath.Join(articleRoot, ".locks")
	if err := os.MkdirAll(lockRoot, 0o755); err != nil {
		return SiteArticlesResult{}, fmt.Errorf("create site article root: %w", err)
	}
	batchLock, err := acquireFileLock(filepath.Join(articleRoot, lockName), g.staleAfter)
	if err != nil {
		return SiteArticlesResult{}, err
	}
	defer batchLock.release()

	type selectedArticle struct {
		column  PreparedColumn
		article model.Article
	}
	selected := make(map[int64]selectedArticle)
	selectedOrder := make([]int64, 0)
	skippedExternal, skippedData, skippedInvalid := 0, 0, 0
	for _, column := range columns {
		for _, article := range column.Articles {
			if article.Type == model.ArticleTypeData {
				skippedData++
				continue
			}
			if _, external, ok := safeHref(strings.TrimSpace(article.URL)); ok && external {
				skippedExternal++
				continue
			}
			articleID, parseErr := strconv.ParseInt(strings.TrimSpace(article.ID), 10, 64)
			if parseErr != nil || articleID <= 0 || article.ColumnID <= 0 || article.PublishTime.IsZero() {
				skippedInvalid++
				continue
			}
			if _, exists := selected[articleID]; !exists {
				selected[articleID] = selectedArticle{column: column, article: article}
				selectedOrder = append(selectedOrder, articleID)
			}
		}
	}
	footerLinks, err := g.fetchFooterLinks(ctx)
	if err != nil {
		return SiteArticlesResult{}, err
	}
	generated := 0
	for _, articleID := range selectedOrder {
		item := selected[articleID]
		lock, err := acquireFileLock(filepath.Join(lockRoot, strconv.FormatInt(articleID, 10)+".lock"), g.staleAfter)
		if err != nil {
			return SiteArticlesResult{}, err
		}
		output, outputErr := g.articleOutput(item.article)
		if outputErr == nil {
			_, outputErr = g.publishPreparedArticle(item.column.Column, item.column.Articles, item.article, footerLinks, item.column.Column.ID, articleID, output)
		}
		lock.release()
		if outputErr != nil {
			return SiteArticlesResult{}, outputErr
		}
		generated++
	}
	generatedAt := time.Now().In(g.location)
	return SiteArticlesResult{
		GeneratedAt: generatedAt, DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000,
		GeneratedFiles: generated, GeneratedDetails: generated, Generated: generated,
		SkippedExternal: skippedExternal, SkippedData: skippedData,
		SkippedInvalid: skippedInvalid, Output: articleRoot,
	}, nil
}

func (g *PageGenerator) generatePreparedList(column model.Column, articles []model.Article, footerLinks []LinkGroupView) (ListResult, error) {
	totalPages := int(math.Ceil(float64(len(articles)) / float64(g.cfg.Site.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	targetParent := filepath.Join(g.cfg.Site.OutputRoot, "list")
	if err := os.MkdirAll(targetParent, 0o755); err != nil {
		return ListResult{}, fmt.Errorf("create prepared list root: %w", err)
	}
	target := filepath.Join(targetParent, strconv.FormatInt(column.ID, 10))
	lock, err := acquireFileLock(target+".lock", g.staleAfter)
	if err != nil {
		return ListResult{}, err
	}
	defer lock.release()
	staging, err := os.MkdirTemp(targetParent, ".column-"+strconv.FormatInt(column.ID, 10)+"-*")
	if err != nil {
		return ListResult{}, fmt.Errorf("create prepared list staging: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(staging)
		}
	}()
	tpl, err := template.ParseFiles(g.cfg.Site.ListTemplate)
	if err != nil {
		return ListResult{}, fmt.Errorf("parse prepared list template: %w", err)
	}
	generatedAt := time.Now().In(g.location)
	for page := 1; page <= totalPages; page++ {
		start := (page - 1) * g.cfg.Site.PageSize
		end := start + g.cfg.Site.PageSize
		if start > len(articles) {
			start = len(articles)
		}
		if end > len(articles) {
			end = len(articles)
		}
		items := make([]InnerArticleView, 0, end-start)
		for _, article := range articles[start:end] {
			item := g.innerArticle(article, column.ID, "../../")
			if !item.ExternalLink {
				item.Href = withListOrigin(item.Href, column.ID, column.Name, page)
			}
			items = append(items, item)
		}
		data := ListPageData{
			GeneratedAt: generatedAt.Format(time.RFC3339), RootPrefix: "../../", ColumnID: column.ID,
			Title: column.Name, Items: items, Page: page, PageSize: g.cfg.Site.PageSize,
			TotalItems: len(articles), TotalPages: totalPages, Pagination: buildPagination(page, totalPages), FooterLinks: footerLinks,
		}
		if page > 1 {
			data.Previous = strconv.Itoa(page-1) + ".html"
		}
		if page < totalPages {
			data.Next = strconv.Itoa(page+1) + ".html"
		}
		var rendered bytes.Buffer
		if err := tpl.Execute(&rendered, data); err != nil {
			return ListResult{}, fmt.Errorf("render prepared list page %d: %w", page, err)
		}
		if err := validateInnerPage(rendered.Bytes(), "class=\"full-news-list\""); err != nil {
			return ListResult{}, err
		}
		if err := os.WriteFile(filepath.Join(staging, strconv.Itoa(page)+".html"), rendered.Bytes(), 0o644); err != nil {
			return ListResult{}, err
		}
	}
	if err := replaceDirectory(staging, target); err != nil {
		return ListResult{}, err
	}
	cleanup = false
	return ListResult{GeneratedAt: generatedAt, ColumnID: column.ID, TotalItems: len(articles), TotalPages: totalPages, PageSize: g.cfg.Site.PageSize, Output: target}, nil
}

// ensureStaticScaffold makes a new deployment directory usable without
// rewriting files that have already been published. Incremental list, article,
// and page generation must only publish their own outputs; shared assets and
// unrelated root pages remain part of the current deployment until a complete
// site generation replaces the directory.
func ensureStaticScaffold(sourceRoot, distRoot string, skipPages map[string]struct{}) error {
	sourceRoot, distRoot = filepath.Clean(sourceRoot), filepath.Clean(distRoot)
	if sourceRoot == distRoot {
		return errors.New("site output_root and dist_root must be different")
	}
	if err := os.MkdirAll(distRoot, 0o755); err != nil {
		return fmt.Errorf("create dist root: %w", err)
	}
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return fmt.Errorf("read static source root: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".html" {
			continue
		}
		if _, skip := skipPages[entry.Name()]; skip {
			continue
		}
		target := filepath.Join(distRoot, entry.Name())
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect static page %s: %w", entry.Name(), err)
		}
		data, err := os.ReadFile(filepath.Join(sourceRoot, entry.Name()))
		if err != nil {
			return fmt.Errorf("read static page %s: %w", entry.Name(), err)
		}
		if err := publishFile(target, data); err != nil {
			return err
		}
	}
	for _, directory := range []string{"assets", "css", "js"} {
		source := filepath.Join(sourceRoot, directory)
		if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			target := filepath.Join(distRoot, directory, relative)
			if entry.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			if _, err := os.Stat(target); err == nil {
				return nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return publishFile(target, data)
		}); err != nil {
			return fmt.Errorf("copy static directory %s: %w", directory, err)
		}
	}
	return nil
}
