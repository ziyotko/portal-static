package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"portal-static/internal/adapters/camie/config"
	"portal-static/internal/adapters/camie/model"
	"portal-static/internal/contracts"
)

const homeNoticeLimit = 8

var titleColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{3}(?:[0-9a-fA-F]{3})?$`)

var updatesFallbackCover = map[string]string{
	"standards-work":         "assets/images/updates-standards-work.jpg",
	"training-international": "assets/images/updates-international.jpg",
	"training-talent":        "assets/images/updates-talent.jpg",
	"policy-research":        "assets/images/updates-policy.jpg",
}

// safeTitleColor 只输出可验证的颜色值，避免后台颜色字段进入内联 CSS 时注入其他声明。
// Element Plus 颜色选择器的旧数据可能是 rgb()/rgba()，不能只识别十六进制。
func safeTitleColor(raw string) template.CSS {
	color := strings.TrimSpace(raw)
	if titleColorPattern.MatchString(color) {
		return template.CSS(color)
	}
	name, values, ok := strings.Cut(color, "(")
	if !ok || !strings.HasSuffix(values, ")") || (name != "rgb" && name != "rgba") {
		return ""
	}
	parts := strings.Split(strings.TrimSuffix(values, ")"), ",")
	want := 3
	if name == "rgba" {
		want = 4
	}
	if len(parts) != want {
		return ""
	}
	var channels [3]int
	for i := range channels {
		value, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil || value < 0 || value > 255 {
			return ""
		}
		channels[i] = value
	}
	if name == "rgb" {
		return template.CSS(fmt.Sprintf("#%02x%02x%02x", channels[0], channels[1], channels[2]))
	}
	alpha, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
	if err != nil || math.IsNaN(alpha) || alpha < 0 || alpha > 1 {
		return ""
	}
	return template.CSS(fmt.Sprintf("rgba(%d,%d,%d,%s)", channels[0], channels[1], channels[2], strconv.FormatFloat(alpha, 'f', -1, 64)))
}

type Source interface {
	FetchPageColumns(context.Context, string, int64) ([]model.Column, error)
	FetchColumnArticles(context.Context, int64) (model.Column, []model.Article, error)
	FetchAttachments(context.Context, int64) ([]model.Attachment, error)
}
type Article struct {
	ID          int64              `json:"id"`
	Title       string             `json:"title"`
	Bold        bool               `json:"-"`
	Color       template.CSS       `json:"-"`
	Summary     string             `json:"summary"`
	Date        string             `json:"date"`
	Href        string             `json:"href"`
	Category    string             `json:"category"`
	ColumnCode  string             `json:"columnCode"`
	Cover       string             `json:"cover"`
	HasCover    bool               `json:"-"`
	Source      string             `json:"source"`
	Video       bool               `json:"video"`
	HasVideo    bool               `json:"-"`
	VideoURL    string             `json:"-"`
	Content     template.HTML      `json:"-"`
	Attachments []model.Attachment `json:"-"`
}
type Link struct {
	Name, Href string
	Active     bool
	Current    bool
	Number     int
	Children   []Link
}
type Group struct {
	Name, Href string
	Items      []*Article
	Feature    *Article
	Rest       []*Article
}
type Branch struct {
	Name, Code, Image string
}
type Partner struct {
	Name, Href, Image, ViewBox  string
	Width, Height, DisplayWidth int
}
type Page struct {
	ActiveNav                             string
	Title, Kind, BodyClass, RootPrefix    string
	StaticBuildID, StaticGeneratedAt      string
	ColumnCode                            string
	ColumnPaths                           template.JS
	ColumnPathHrefs                       template.JS
	DetailContexts                        template.JS
	Items                                 []*Article
	Article, PreviousArticle, NextArticle *Article
	Parent                                *Link
	Breadcrumbs                           []Link
	Side, Pages                           []Link
	Hero                                  []*Article
	Notices, Topics                       []Group
	Experts                               Group
	DataCenterURL                         string
	ConferenceBannerURL                   string
	MemberLoginPath, MemberRegisterPath   string
	Branches                              []Branch
	BranchIntro                           bool
	PartnerRows                           [][]Partner
	Total, PageSize, Page, TotalPages     int
	Previous, Next, BackHref, PagePrefix  string
}
type Result struct {
	Output      string    `json:"output"`
	Files       int       `json:"files"`
	Articles    int       `json:"articles"`
	Lists       int       `json:"lists"`
	GeneratedAt time.Time `json:"generated_at"`
}

type detailArticleLink struct {
	Title string `json:"title"`
	Href  string `json:"href"`
}

type detailContext struct {
	Sidebar  string             `json:"sidebar"`
	Previous *detailArticleLink `json:"previous,omitempty"`
	Next     *detailArticleLink `json:"next,omitempty"`
}
type Generator struct {
	cfg    config.Config
	source Source
	mu     sync.Mutex
}
type catalog struct {
	columns []model.Column
	groups  map[int64][]*Article
	byID    map[int64]*Article
	byName  map[string]int64
	byCode  map[string]int64
	all     []*Article
	raw     map[int64]model.Article
}

func New(cfg config.Config, source Source) *Generator { return &Generator{cfg: cfg, source: source} }

func (g *Generator) load(ctx context.Context) (*catalog, error) {
	c := &catalog{groups: map[int64][]*Article{}, byID: map[int64]*Article{}, byName: map[string]int64{}, byCode: map[string]int64{}, raw: map[int64]model.Article{}}
	queue := []int64{0}
	var homeRootIDs []int64
	seen := map[int64]bool{}
	restricted := map[int64]bool{}
	zone, _ := time.LoadLocation(g.cfg.Site.Timezone)
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		columns, err := g.source.FetchPageColumns(ctx, g.cfg.Site.PageName, parent)
		if err != nil {
			return nil, err
		}
		for _, col := range columns {
			if isDuplicateHomeColumnCode(col.Code) {
				continue
			}
			if col.ID <= 0 || seen[col.ID] {
				return nil, fmt.Errorf("invalid or cyclic column %d", col.ID)
			}
			col.Code = strings.TrimSpace(col.Code)
			if col.Code == "" {
				return nil, fmt.Errorf("column %d (%s) has no stable code", col.ID, col.Name)
			}
			if _, exists := c.byCode[col.Code]; exists {
				return nil, fmt.Errorf("duplicate column code %q", col.Code)
			}
			// Expert articles and the dedicated homepage branch lists have their
			// own public navigation, even when the CMS nests them under About/Home.
			if col.Code == "about-experts" || col.Code == "home-fenzhijigou" {
				col.ParentID = 0
			}
			seen[col.ID] = true
			if _, exists := c.byName[col.Name]; !exists {
				c.byName[col.Name] = col.ID
			}
			c.byCode[col.Code] = col.ID
			c.columns = append(c.columns, col)
			if col.Code == "home" {
				// Load the homepage placements after public columns so shared
				// articles keep their public section as the canonical category.
				homeRootIDs = append(homeRootIDs, col.ID)
				continue
			}
			queue = append(queue, col.ID)
			_, items, err := g.source.FetchColumnArticles(ctx, col.ID)
			if err != nil {
				return nil, err
			}
			sort.SliceStable(items, func(i, j int) bool {
				a, b := items[i], items[j]
				if a.IsTop != b.IsTop {
					return a.IsTop
				}
				if !a.PublishTime.Equal(b.PublishTime) {
					return a.PublishTime.After(b.PublishTime)
				}
				return a.ID > b.ID
			})
			if isPrivateColumnCode(col.Code) {
				for _, article := range items {
					restricted[article.ID] = true
				}
				continue
			}
			ids := map[int64]bool{}
			for _, a := range items {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if a.ID <= 0 || ids[a.ID] || !model.IsSupportedArticleType(a.Type) || a.PublishTime.IsZero() || a.PublishTime.After(time.Now()) {
					continue
				}
				ids[a.ID] = true
				a.PublishTime = a.PublishTime.In(zone)
				href := fmt.Sprintf("article/%s/%d.html", a.PublishTime.Format("2006/01"), a.ID)
				if a.URL != "" {
					u, err := url.Parse(a.URL)
					if err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
						href = u.String()
					}
				}
				cover := g.media(a.Cover)
				// 老数据可能已把通用占位图写进 cover；首页 updates 仍应视为无专属封面。
				hasCover := cover != "" && cover != g.cfg.Site.FallbackCover && cover != "assets/images/hero-building.png"
				if cover == "" {
					cover = g.cfg.Site.FallbackCover
				}
				color := safeTitleColor(a.DefaultColor)
				v := &Article{ID: a.ID, Title: a.Title, Bold: a.IsBold, Color: color, Summary: a.Summary, Category: col.Name, ColumnCode: col.Code, Source: a.Source, Date: a.PublishTime.Format("2006-01-02"), Href: href, Cover: cover, HasCover: hasCover, Video: a.Type == 2}
				if existing, ok := c.byID[a.ID]; ok {
					copy := *existing
					copy.Category, copy.ColumnCode = col.Name, col.Code
					copy.Bold, copy.Color = a.IsBold, color
					copy.Href, copy.Source = href, a.Source
					copy.HasCover = hasCover
					if col.Code == "home-hero" {
						// A shared article may have reached c.byID through an old
						// publication snapshot. The carousel displays current article
						// fields from its own query.
						copy.Title, copy.Summary = a.Title, a.Summary
						copy.Date, copy.Cover, copy.Video = a.PublishTime.Format("2006-01-02"), cover, a.Type == 2
					}
					v = &copy
				} else {
					attachments, err := g.source.FetchAttachments(ctx, a.ID)
					if err != nil {
						return nil, err
					}
					for _, attachment := range attachments {
						attachment.URL = g.media(attachment.URL)
						if attachment.URL != "" {
							v.Attachments = append(v.Attachments, attachment)
						}
					}
					c.byID[a.ID] = v
					c.raw[a.ID] = a
					c.all = append(c.all, v)
				}
				c.groups[col.ID] = append(c.groups[col.ID], v)
			}
		}
		if len(queue) == 0 && len(homeRootIDs) > 0 {
			queue, homeRootIDs = homeRootIDs, nil
		}
	}
	if len(restricted) > 0 {
		filtered := c.all[:0]
		for _, article := range c.all {
			if restricted[article.ID] {
				delete(c.byID, article.ID)
				delete(c.raw, article.ID)
				continue
			}
			filtered = append(filtered, article)
		}
		c.all = filtered
		for id, items := range c.groups {
			kept := items[:0]
			for _, article := range items {
				if !restricted[article.ID] {
					kept = append(kept, article)
				}
			}
			c.groups[id] = kept
		}
	}
	sort.SliceStable(c.all, func(i, j int) bool {
		a, b := c.raw[c.all[i].ID], c.raw[c.all[j].ID]
		if a.IsTop != b.IsTop {
			return a.IsTop
		}
		if !a.PublishTime.Equal(b.PublishTime) {
			return a.PublishTime.After(b.PublishTime)
		}
		return a.ID > b.ID
	})
	return c, nil
}

func isDuplicateHomeColumnCode(code string) bool {
	switch code {
	case "home-news-notice", "home-news-association", "home-news-member",
		"home-training-meetings", "home-standards-work", "home-standards-innovation",
		"home-training-international", "home-training-talent", "home-policy-reports", "home-experts":
		return true
	default:
		return false
	}
}

func root(p Page, raw string) string {
	if raw == "" || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "#") {
		return raw
	}
	u, err := url.Parse(raw)
	if err == nil && u.IsAbs() {
		return raw
	}
	return p.RootPrefix + strings.TrimPrefix(raw, "./")
}
func (g *Generator) media(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.IsAbs() {
		if (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			return raw
		}
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		return ""
	}
	clean := strings.TrimPrefix(raw, "./")
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}
	// Project-owned graphics always stay local, even when an upload host is configured.
	if strings.HasPrefix(clean, "assets/") || strings.HasPrefix(clean, "slice/") {
		return clean
	}
	if g.cfg.Site.MediaBaseURL != "" {
		return strings.TrimRight(g.cfg.Site.MediaBaseURL, "/") + "/" + strings.TrimLeft(clean, "/")
	}
	return clean
}
func (g *Generator) content(raw, prefix string) template.HTML {
	policy := bluemonday.UGCPolicy()
	policy.AllowElements("video", "source", "section")
	policy.AllowAttrs("src", "poster", "controls", "preload", "width", "height").OnElements("video")
	policy.AllowAttrs("src", "type").OnElements("source")
	policy.AllowAttrs("data-text-align").Matching(regexp.MustCompile(`^(?:left|center|right|justify)$`)).OnElements("p", "div", "section", "h1", "h2", "h3", "h4", "h5", "h6", "td", "th", "img")
	policy.AllowRelativeURLs(true)
	policy.AllowURLSchemes("http", "https")
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				var alignment string
				switch a.Key {
				case "align":
					alignment = strings.ToLower(strings.TrimSpace(a.Val))
				case "style":
					for _, declaration := range strings.Split(a.Val, ";") {
						property, value, ok := strings.Cut(declaration, ":")
						if ok && strings.EqualFold(strings.TrimSpace(property), "text-align") {
							alignment = strings.ToLower(strings.TrimSpace(value))
						}
					}
				}
				if alignment == "left" || alignment == "center" || alignment == "right" || alignment == "justify" {
					n.Attr = append(n.Attr, html.Attribute{Key: "data-text-align", Val: alignment})
				}
			}
		}
		for i, a := range n.Attr {
			if a.Key == "src" || a.Key == "poster" || a.Key == "href" {
				if strings.HasPrefix(a.Val, "#") {
					continue
				}
				media := g.media(a.Val)
				n.Attr[i].Val = root(Page{RootPrefix: prefix}, media)
			}
		}
		if n.Type == html.ElementNode && n.Data == "video" {
			n.Attr = append(n.Attr, html.Attribute{Key: "controls", Val: "controls"}, html.Attribute{Key: "preload", Val: "metadata"})
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	var b bytes.Buffer
	_ = html.Render(&b, doc)
	return template.HTML(policy.Sanitize(b.String()))
}

func playableVideoAttachment(attachment model.Attachment) bool {
	for _, raw := range []string{attachment.Name, attachment.URL} {
		parsed, err := url.Parse(raw)
		if err == nil && strings.EqualFold(filepath.Ext(parsed.Path), ".mp4") {
			return true
		}
	}
	return false
}

func isPrivateColumnCode(code string) bool {
	return code == "member" || strings.HasPrefix(code, "member-") || code == "videos-members"
}

func publicChildColumns(c *catalog, parentID int64) []model.Column {
	children := make([]model.Column, 0)
	for _, column := range c.columns {
		if column.ParentID == parentID && !isPrivateColumnCode(column.Code) {
			children = append(children, column)
		}
	}
	sort.SliceStable(children, func(i, j int) bool {
		if children[i].Sort != children[j].Sort {
			return children[i].Sort < children[j].Sort
		}
		return children[i].ID < children[j].ID
	})
	return children
}

func groupByID(c *catalog, id int64, limit int) Group {
	name, code := "", ""
	displayID := id
	var selected model.Column
	for _, column := range c.columns {
		if column.ID == id {
			name = column.Name
			code = column.Code
			selected = column
			break
		}
	}
	// 一级栏目是导航容器：它不读取自身文章，默认落到第一个公开二级栏目。
	if selected.ID != 0 && ((selected.ParentID == 0 && selected.Code != "home") || selected.Code == "about-branches") {
		if children := publicChildColumns(c, selected.ID); len(children) > 0 {
			displayID = children[0].ID
		}
	}
	// 二级、三级栏目只展示直接发布在当前栏目的文章，不汇总后代栏目。
	// Keep each publication's current-column context when an article is cross-posted.
	// Each column is ordered by its own publication's top/date values.
	// A shared article can have a different priority in another column, so the
	// globally sorted catalog cannot determine this list's order.
	items := make([]*Article, 0, len(c.groups[displayID]))
	seen := make(map[int64]bool, len(c.groups[displayID]))
	for _, article := range c.groups[displayID] {
		if !seen[article.ID] {
			seen[article.ID] = true
			items = append(items, article)
		}
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	href := "news.html"
	switch code {
	case "home", "home-hero":
		href = "index.html"
	default:
		if displayID > 0 {
			href = fmt.Sprintf("list/%d/1.html", displayID)
		}
	}
	out := Group{Name: name, Href: href, Items: items}
	if len(items) > 0 {
		out.Feature = items[0]
		out.Rest = items[1:]
	}
	return out
}
func group(c *catalog, selector string, limit int) Group {
	id := c.byCode[selector]
	if id == 0 {
		id = c.byName[selector]
	}
	return groupByID(c, id, limit)
}

func defaultListColumn(c *catalog, id int64) model.Column {
	var selected model.Column
	for _, column := range c.columns {
		if column.ID == id {
			selected = column
			break
		}
	}
	if selected.ID == 0 || (selected.ParentID != 0 && selected.Code != "about-branches") || selected.Code == "home" {
		return selected
	}
	if children := publicChildColumns(c, selected.ID); len(children) > 0 {
		return children[0]
	}
	return selected
}

func topColumn(c *catalog, code string) model.Column {
	id := c.byCode[code]
	columns := make(map[int64]model.Column, len(c.columns))
	for _, column := range c.columns {
		columns[column.ID] = column
	}
	column := columns[id]
	for column.ParentID != 0 {
		column = columns[column.ParentID]
	}
	return column
}

func side(c *catalog, currentCode string) []Link {
	top := topColumn(c, currentCode)
	if top.ID == 0 {
		return nil
	}
	if top.Code == "home" {
		return nil
	}
	columns := make(map[int64]model.Column, len(c.columns))
	for _, column := range c.columns {
		columns[column.ID] = column
	}
	activeID := c.byCode[currentCode]
	if activeID == top.ID {
		activeID = 0
		if children := publicChildColumns(c, top.ID); len(children) > 0 {
			activeID = children[0].ID
		}
	} else {
		for activeID != 0 && columns[activeID].ParentID != top.ID {
			activeID = columns[activeID].ParentID
		}
	}
	var links []Link
	for _, column := range publicChildColumns(c, top.ID) {
		active := column.ID == activeID
		link := Link{Name: column.Name, Href: groupByID(c, column.ID, 0).Href, Active: active, Current: column.Code == currentCode}
		for _, child := range publicChildColumns(c, column.ID) {
			link.Children = append(link.Children, Link{Name: child.Name, Href: groupByID(c, child.ID, 0).Href, Active: child.Code == currentCode, Current: child.Code == currentCode})
		}
		links = append(links, link)
	}
	if len(links) == 0 {
		links = append(links, Link{Name: top.Name, Href: groupByID(c, top.ID, 0).Href, Active: true, Current: top.Code == currentCode})
	}
	if top.Code == "videos" {
		links = append(links, Link{Name: "会员专享", Href: "pages/member.html?column=会员专享&mode=video"})
	}
	return links
}

func activeNav(c *catalog, p Page) string {
	if p.ColumnCode == "about-english" {
		return ""
	}
	if p.Kind == "home" {
		return "首页"
	}
	if p.Kind == "about" {
		return "关于协会"
	}
	code := p.ColumnCode
	if p.Article != nil {
		code = p.Article.ColumnCode
	}
	switch topColumn(c, code).Code {
	case "party":
		return "党建专栏"
	case "ministry", "policy-research":
		return "部委动态"
	case "news":
		return "新闻中心"
	case "training":
		return "交流培训"
	case "standards":
		return "科技标准"
	case "about":
		return "关于协会"
	case "about-experts":
		return ""
	case "home-fenzhijigou":
		return "首页"
	default:
		return "首页"
	}
}

func (g *Generator) GenerateSite(ctx context.Context) (result Result, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return result, err
	}
	target := g.cfg.Site.DistRoot
	if requested := strings.TrimSpace(contracts.OptionsFrom(ctx).OutputPath); requested != "" {
		if err = g.ValidateOutputPath(requested); err != nil {
			return result, err
		}
		target = requested
	}
	if err = config.ValidateOutput(g.cfg.Site.SourceRoot, target); err != nil {
		return result, err
	}
	if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return result, err
	}
	// No stale timeout: a long-running live publisher must never lose its lock.
	lock, err := os.OpenFile(target+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return result, fmt.Errorf("publisher is locked (check %s.lock): %w", target, err)
	}
	_, _ = fmt.Fprintf(lock, "pid=%d\nstarted=%s\n", os.Getpid(), time.Now().Format(time.RFC3339))
	_ = lock.Close()
	defer os.Remove(target + ".lock")
	generatedAt := time.Now()
	if location, loadErr := time.LoadLocation(g.cfg.Site.Timezone); loadErr == nil {
		generatedAt = generatedAt.In(location)
	}
	buildID := generatedAt.Format("20060102T150405.000000000-0700")
	c, err := g.load(ctx)
	if err != nil {
		return result, err
	}
	columnPaths := make(map[string][]string, len(c.columns))
	columnPathHrefs := make(map[string][]string, len(c.columns))
	columnsByID := make(map[int64]model.Column, len(c.columns))
	for _, column := range c.columns {
		columnsByID[column.ID] = column
	}
	for _, column := range c.columns {
		if isPrivateColumnCode(column.Code) {
			continue
		}
		var path []string
		var hrefs []string
		for current := column; current.ID != 0; current = columnsByID[current.ParentID] {
			if current.Code != "home" {
				path = append(path, current.Name)
				href := fmt.Sprintf("list/%d/1.html", current.ID)
				if current.Code == "home-hero" {
					href = "index.html"
				}
				hrefs = append(hrefs, href)
			}
		}
		for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
			path[left], path[right] = path[right], path[left]
			hrefs[left], hrefs[right] = hrefs[right], hrefs[left]
		}
		columnPaths[column.Code] = path
		columnPathHrefs[column.Code] = hrefs
	}
	columnPathJSON, err := json.Marshal(columnPaths)
	if err != nil {
		return result, err
	}
	columnPathHrefJSON, err := json.Marshal(columnPathHrefs)
	if err != nil {
		return result, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".camie-stage-")
	if err != nil {
		return result, err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	if err = copyAssets(g.cfg.Site.SourceRoot, stage); err != nil {
		return result, err
	}
	funcs := template.FuncMap{
		"root": root, "add": func(a, b int) int { return a + b },
		"columnURL": func(p Page, code string) string { return root(p, group(c, code, 0).Href) },
		"articleURL": func(p Page, id int) string {
			if a := c.byID[int64(id)]; a != nil {
				return root(p, a.Href)
			}
			return root(p, "news.html")
		},
	}
	templates, err := template.New("portal").Funcs(funcs).ParseGlob(filepath.Join(g.cfg.Site.TemplateRoot, "*.tmpl"))
	if err != nil {
		return result, err
	}
	render := func(file, kind string, p Page) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		p.ActiveNav = activeNav(c, p)
		p.StaticBuildID = buildID
		p.StaticGeneratedAt = generatedAt.Format(time.RFC3339Nano)
		p.ColumnPaths = template.JS(columnPathJSON)
		p.ColumnPathHrefs = template.JS(columnPathHrefJSON)
		currentColumn := p.ColumnCode
		if p.Article != nil {
			currentColumn = p.Article.ColumnCode
		}
		top := topColumn(c, currentColumn)
		if top.ID != 0 && top.Code != currentColumn && top.Code != "home" {
			parent := Link{Name: top.Name, Href: groupByID(c, top.ID, 0).Href}
			p.Parent = &parent
		}
		// Build the complete root-to-leaf column chain so second-, third-, and
		// deeper-level pages keep every breadcrumb segment. Flat legacy/demo
		// columns fall back to the navigation group used elsewhere on the site.
		columns := make(map[int64]model.Column, len(c.columns))
		for _, col := range c.columns {
			columns[col.ID] = col
		}
		var ancestors []Link
		if id := c.byCode[currentColumn]; id != 0 {
			for parentID := columns[id].ParentID; parentID != 0; {
				col, ok := columns[parentID]
				if !ok {
					break
				}
				if col.Code != "home" {
					ancestors = append(ancestors, Link{Name: col.Name, Href: groupByID(c, col.ID, 0).Href})
				}
				parentID = col.ParentID
			}
			for left, right := 0, len(ancestors)-1; left < right; left, right = left+1, right-1 {
				ancestors[left], ancestors[right] = ancestors[right], ancestors[left]
			}
		}
		p.Breadcrumbs = ancestors
		if p.ActiveNav == "党建专栏" {
			p.BodyClass += " party-page"
		}
		p.RootPrefix = strings.Repeat("../", strings.Count(file, "/"))
		p.MemberLoginPath = g.cfg.Site.MemberLoginPath
		p.MemberRegisterPath = g.cfg.Site.MemberRegisterPath
		if p.RootPrefix == "" {
			p.RootPrefix = "./"
		}
		if p.Article != nil {
			v := *p.Article
			v.Content = g.content(c.raw[v.ID].Content, p.RootPrefix)
			v.HasVideo = strings.Contains(string(v.Content), "<video")
			if v.Video {
				for _, attachment := range v.Attachments {
					if playableVideoAttachment(attachment) {
						v.VideoURL = attachment.URL
						break
					}
				}
			}
			p.Article = &v
		}
		var b bytes.Buffer
		if err := templates.ExecuteTemplate(&b, kind, p); err != nil {
			return fmt.Errorf("render %s: %w", file, err)
		}
		result.Files++
		return writeFile(stage, file, b.Bytes())
	}
	home := Page{
		Title:     "首页",
		Kind:      "home",
		BodyClass: "home",
		Hero:      group(c, g.cfg.Site.HeroColumn, 0).Items,
		Branches: []Branch{
			{Name: "水分会", Code: "main-branch-water", Image: "assets/images/branch-water.png"},
			{Name: "大气分会", Code: "main-branch-atmosphere", Image: "assets/images/branch-atmosphere.png"},
			{Name: "固废分会", Code: "main-branch-solid-waste", Image: "assets/images/branch-solid-waste.png"},
			{Name: "环境监测分会", Code: "main-branch-monitoring", Image: "assets/images/branch-monitoring.png"},
			{Name: "噪声分会", Code: "main-branch-noise", Image: "assets/images/branch-noise.png"},
			{Name: "紫外线分会", Code: "main-branch-uv", Image: "assets/images/branch-uv.png"},
			{Name: "臭氧分会", Code: "main-branch-ozone", Image: "assets/images/branch-odor.png"},
			{Name: "人工智能分会", Code: "main-branch-ai", Image: "assets/images/branch-ai.png"},
			{Name: "环境工程分会", Code: "main-branch-engineering", Image: "assets/images/branch-engineering.png"},
		},
		PartnerRows: [][]Partner{
			{
				{Name: "南京大学环境学院", Href: "https://www.nju.edu.cn/", Image: "assets/images/partner-logos/nanjing-university.png", ViewBox: "12 8 219 50", Width: 244, Height: 66, DisplayWidth: 219},
				{Name: "紫金龙净环保新能源股份有限公司", Href: "https://www.longking.com.cn/", Image: "assets/images/partner-logos/zijin-longking.png", ViewBox: "0 0 183 48", Width: 183, Height: 48, DisplayWidth: 183},
				{Name: "北京城市排水集团有限责任公司", Href: "https://www.bdc.cn/", Image: "assets/images/partner-logos/beijing-drainage.png", ViewBox: "41 7 158 51", Width: 244, Height: 66, DisplayWidth: 158},
				{Name: "苏州帝瀚环保科技股份有限公司", Href: "https://www.dihillgreen.com/", Image: "assets/images/partner-logos/dihill.png", ViewBox: "58 18 136 30", Width: 244, Height: 66, DisplayWidth: 136},
				{Name: "江苏一环集团有限公司", Href: "http://www.yihuan.com/", Image: "assets/images/partner-logos/jiangsu-yihuan.png", ViewBox: "9 13 224 40", Width: 244, Height: 66, DisplayWidth: 224},
				{Name: "河南康宁特环保科技股份有限公司", Href: "https://www.knthb.com/", Image: "assets/images/partner-logos/kangningte.png", ViewBox: "61 7 122 52", Width: 244, Height: 66, DisplayWidth: 122},
				{Name: "龙净环保股份有限公司", Href: "https://www.longking.com.cn/", Image: "assets/images/partner-logos/longking.png", ViewBox: "58 11 129 44", Width: 244, Height: 66, DisplayWidth: 129},
			},
			{
				{Name: "中国天楹股份有限公司", Href: "https://www.cnty.cn/", Image: "assets/images/partner-logos/cnty.png", ViewBox: "48 10 148 46", Width: 244, Height: 66, DisplayWidth: 148},
				{Name: "杰瑞新能源再生循环科技有限公司", Href: "https://www.jereh.com/cn/", Image: "assets/images/partner-logos/jereh-recycling.png", ViewBox: "5 10 166 60", Width: 178, Height: 80, DisplayWidth: 166},
				{Name: "合肥通用机械研究院有限公司", Href: "http://www.hgmri.com/", Image: "assets/images/partner-logos/hefei-general-machinery.png", ViewBox: "17 18 209 29", Width: 244, Height: 66, DisplayWidth: 209},
				{Name: "长江生态环保集团有限公司", Href: "https://www.yeec.com.cn/", Image: "assets/images/partner-logos/yangtze-ecology.png", ViewBox: "0 0 418 55", Width: 418, Height: 55, DisplayWidth: 350},
				{Name: "科林环保技术有限责任公司", Href: "https://www.kelin-china.com/", Image: "assets/images/partner-logos/kelin.png", ViewBox: "55 11 136 44", Width: 244, Height: 66, DisplayWidth: 136},
				{Name: "中车产业投资有限公司", Href: "https://www.crrcgc.cc/cytz/277_19585/index.html", Image: "assets/images/partner-logos/crrc-investment.png", ViewBox: "0 0 169 55", Width: 169, Height: 55, DisplayWidth: 169},
			},
		},
	}
	for _, n := range []string{"news-notice", "news-association", "news-member"} {
		home.Notices = append(home.Notices, group(c, n, homeNoticeLimit))
	}
	for _, topic := range []string{
		"training-meetings",
		"standards-work",
		"standards-innovation",
		"training-international",
		"training-talent",
		"policy-research",
	} {
		topicGroup := group(c, topic, 6)
		if topicGroup.Feature != nil && !topicGroup.Feature.HasCover {
			if fallback := updatesFallbackCover[topic]; fallback != "" {
				feature := *topicGroup.Feature
				feature.Cover = fallback
				topicGroup.Feature = &feature
			}
		}
		home.Topics = append(home.Topics, topicGroup)
	}
	dataCenter := group(c, "policy-data", 1)
	home.DataCenterURL = dataCenter.Href
	if dataCenter.Feature != nil {
		home.DataCenterURL = dataCenter.Feature.Href
	}
	home.Experts = group(c, "about-expert-insights", 3)
	conference := group(c, "training-meetings-conference", 0)
	home.ConferenceBannerURL = conference.Href
	for _, article := range conference.Items {
		if article.ID == 20163 {
			home.ConferenceBannerURL = article.Href + "?from=training-meetings-conference"
			break
		}
	}
	if err = render("index.html", "home", home); err != nil {
		return result, err
	}
	makeLists := func(name, code, dir, alias string, items []*Article) error {
		listKind := "list"
		if top := topColumn(c, code); top.ID != 0 {
			candidate := top.Code + "-list"
			if templates.Lookup(candidate) != nil {
				listKind = candidate
			}
		}
		totalPages := (len(items) + g.cfg.Site.PageSize - 1) / g.cfg.Site.PageSize
		if totalPages < 1 {
			totalPages = 1
		}
		for pageNum := 1; pageNum <= totalPages; pageNum++ {
			start := (pageNum - 1) * g.cfg.Site.PageSize
			end := start + g.cfg.Site.PageSize
			if end > len(items) {
				end = len(items)
			}
			p := Page{Title: name, ColumnCode: code, Kind: "list", BodyClass: "list-page", Items: items[start:end], Side: side(c, code), Total: len(items), PageSize: g.cfg.Site.PageSize, Page: pageNum, TotalPages: totalPages, PagePrefix: dir + "/"}
			if pageNum > 1 {
				p.Previous = fmt.Sprintf("%s/%d.html", dir, pageNum-1)
			}
			if pageNum < totalPages {
				p.Next = fmt.Sprintf("%s/%d.html", dir, pageNum+1)
			}
			// Bounded page controls; all pages remain reachable through previous/next links.
			lastPage := 0
			for n := 1; n <= totalPages; n++ {
				if n == 1 || n == totalPages || (n >= pageNum-2 && n <= pageNum+2) {
					if lastPage != 0 && n > lastPage+1 {
						p.Pages = append(p.Pages, Link{})
					}
					p.Pages = append(p.Pages, Link{Number: n, Href: fmt.Sprintf("%s/%d.html", dir, n), Active: n == pageNum})
					lastPage = n
				}
			}
			if err := render(fmt.Sprintf("%s/%d.html", dir, pageNum), listKind, p); err != nil {
				return err
			}
			result.Lists++
			if pageNum == 1 && alias != "" {
				if err := render(alias, listKind, p); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, col := range c.columns {
		if isPrivateColumnCode(col.Code) || col.Code == "home" || (strings.HasPrefix(col.Code, "home-") && col.Code != "home-fenzhijigou") {
			continue
		}
		if col.Code == "about" || col.Code == "about-introduction" || col.Code == "about-organization" || col.Code == "about-charter" || col.Code == "about-english" || col.Code == "about-branches" || strings.HasPrefix(col.Code, "branch-") {
			displayColumn := defaultListColumn(c, col.ID)
			bodyClass := "about-page"
			if col.Code == "about-english" {
				bodyClass += " english-about-page"
			}
			p := Page{Title: displayColumn.Name, ColumnCode: displayColumn.Code, Kind: "about", BodyClass: bodyClass, Side: side(c, displayColumn.Code), BranchIntro: strings.HasPrefix(displayColumn.Code, "branch-")}
			items := c.groups[displayColumn.ID]
			for _, a := range items {
				if !a.Video && !strings.HasPrefix(a.Href, "http") {
					p.Article = a
					break
				}
			}
			if err = render(fmt.Sprintf("list/%d/1.html", col.ID), "about", p); err != nil {
				return result, err
			}
			if alias := columnAlias(col.Code); alias != "" {
				if err = render(alias, "about", p); err != nil {
					return result, err
				}
			}
			result.Lists++
			continue
		}
		displayColumn := defaultListColumn(c, col.ID)
		alias := columnAlias(col.Code)
		if err = makeLists(displayColumn.Name, displayColumn.Code, fmt.Sprintf("list/%d", col.ID), alias, groupByID(c, col.ID, 0).Items); err != nil {
			return result, err
		}
	}
	news := group(c, "news", 0).Items
	if len(news) == 0 {
		for _, a := range c.all {
			if !a.Video {
				news = append(news, a)
			}
		}
	}
	var videos []*Article
	seenVideos := make(map[int64]bool)
	for _, a := range c.groups[c.byCode["videos-news"]] {
		if a.Video && !seenVideos[a.ID] {
			seenVideos[a.ID] = true
			videos = append(videos, a)
		}
	}
	if err = makeLists("新闻中心", "news", "list/news", "news.html", news); err != nil {
		return result, err
	}
	if err = makeLists("视频专区", "videos", "list/videos", "videos.html", videos); err != nil {
		return result, err
	}
	firstNews := (*Article)(nil)
	firstVideo := (*Article)(nil)
	if len(videos) > 0 {
		firstVideo = videos[0]
	}
	articleColumns := make(map[int64][]model.Column)
	for _, column := range c.columns {
		for _, publication := range c.groups[column.ID] {
			articleColumns[publication.ID] = append(articleColumns[publication.ID], column)
		}
	}
	for _, a := range c.all {
		if strings.HasPrefix(a.Href, "http") {
			continue
		}
		p := Page{Title: a.Title, ColumnCode: a.ColumnCode, Kind: "article", BodyClass: "article-page", Article: a, Side: side(c, a.ColumnCode), BackHref: group(c, a.ColumnCode, 0).Href}
		contexts := make(map[string]detailContext)
		for _, column := range articleColumns[a.ID] {
			links := side(c, column.Code)
			var sidebar bytes.Buffer
			if len(links) > 0 {
				if err = templates.ExecuteTemplate(&sidebar, "sidebar", Page{Side: links, RootPrefix: strings.Repeat("../", strings.Count(a.Href, "/"))}); err != nil {
					return result, err
				}
			}
			context := detailContext{Sidebar: sidebar.String()}
			siblings := c.groups[column.ID]
			for i, sibling := range siblings {
				if sibling.ID != a.ID {
					continue
				}
				if i > 0 {
					context.Previous = &detailArticleLink{Title: siblings[i-1].Title, Href: siblings[i-1].Href}
				}
				if i+1 < len(siblings) {
					context.Next = &detailArticleLink{Title: siblings[i+1].Title, Href: siblings[i+1].Href}
				}
				break
			}
			contexts[column.Code] = context
		}
		contextJSON, marshalErr := json.Marshal(contexts)
		if marshalErr != nil {
			return result, marshalErr
		}
		p.DetailContexts = template.JS(contextJSON)
		if a.ColumnCode == "home-hero" {
			p.BodyClass += " home-placement-detail"
		}
		if a.Video {
			p.Kind = "video"
			p.BodyClass = "video-page"
		} else if firstNews == nil {
			firstNews = a
		}
		siblings := c.groups[c.byCode[a.ColumnCode]]
		for i, v := range siblings {
			if v.ID == a.ID {
				if i > 0 {
					p.PreviousArticle = siblings[i-1]
				}
				if i+1 < len(siblings) {
					p.NextArticle = siblings[i+1]
				}
				break
			}
		}
		if err = render(a.Href, "article", p); err != nil {
			return result, err
		}
		result.Articles++
	}
	if a := c.byID[102]; a != nil && !a.Video && !strings.HasPrefix(a.Href, "http") {
		firstNews = a
	}
	for _, alias := range []struct {
		file, kind string
		a          *Article
	}{{"detail.html", "article", firstNews}, {"video-detail.html", "video", firstVideo}} {
		p := Page{Title: "内容详情", Kind: alias.kind, BodyClass: "article-page", Article: alias.a, Side: side(c, "news")}
		kind := "article"
		if alias.a == nil {
			kind = "empty-detail"
		} else {
			p.Title = alias.a.Title
			p.ColumnCode = alias.a.ColumnCode
			p.BackHref = group(c, alias.a.ColumnCode, 0).Href
			if alias.a.Video {
				p.BodyClass = "video-page"
			}
		}
		if err = render(alias.file, kind, p); err != nil {
			return result, err
		}
	}
	if err = render("search.html", "search", Page{Title: "搜索资讯", ColumnCode: "news", Kind: "search", BodyClass: "list-page", Side: side(c, "news"), Items: c.all, Total: len(c.all)}); err != nil {
		return result, err
	}
	if err = render("pages/member.html", "member-shell", Page{Title: "会员中心", ColumnCode: "member", Kind: "member", BodyClass: "list-page"}); err != nil {
		return result, err
	}
	if err = render("pages/member-detail.html", "member-detail-shell", Page{Title: "会员内容", ColumnCode: "member", Kind: "member-detail", BodyClass: "article-page"}); err != nil {
		return result, err
	}
	paths := map[string]string{}
	columns := map[string]string{}
	for _, a := range c.all {
		paths[strconv.FormatInt(a.ID, 10)] = a.Href
	}
	for code := range c.byCode {
		if !isPrivateColumnCode(code) {
			columns[code] = group(c, code, 0).Href
		}
	}
	content := struct {
		Generated    bool              `json:"generated"`
		Articles     []*Article        `json:"articles"`
		ArticlePaths map[string]string `json:"articlePaths"`
		Columns      map[string]string `json:"columns"`
		DefaultVideo string            `json:"defaultVideo"`
	}{true, c.all, paths, columns, "video-detail.html"}
	if firstVideo != nil {
		content.DefaultVideo = firstVideo.Href
	}
	payload, err := json.Marshal(content)
	if err != nil {
		return result, err
	}
	if err = writeFile(stage, "generated-content.js", append([]byte("window.CAMIE_STATIC_CONTENT = "), append(payload, []byte(";\n")...)...)); err != nil {
		return result, err
	}
	result.Files++
	if contracts.OptionsFrom(ctx).Grayscale {
		if err = applyGrayscaleToHTMLTree(stage); err != nil {
			return result, err
		}
	}
	if err = validateSite(stage); err != nil {
		return result, err
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if err = publishDirectory(g.cfg.Site.SourceRoot, stage, target); err != nil {
		return result, err
	}
	result.Output = target
	result.GeneratedAt = generatedAt
	return result, nil
}

func columnAlias(code string) string {
	aliases := map[string]string{
		"party": "pages/party.html", "ministry": "pages/ministry.html", "news": "pages/news.html",
		"training": "pages/training.html", "standards": "pages/standards.html", "about": "pages/about.html",
		"policy-research": "pages/policy.html", "about-experts": "pages/experts.html", "videos": "pages/videos.html",
		"about-english": "en/about.html",
	}
	return aliases[code]
}
