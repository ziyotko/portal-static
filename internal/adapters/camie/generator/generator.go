package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
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

type Source interface {
	FetchPageColumns(context.Context, string, int64) ([]model.Column, error)
	FetchColumnArticles(context.Context, int64) (model.Column, []model.Article, error)
	FetchAttachments(context.Context, int64) ([]model.Attachment, error)
}
type Article struct {
	ID          int64              `json:"id"`
	Title       string             `json:"title"`
	Summary     string             `json:"summary"`
	Date        string             `json:"date"`
	Href        string             `json:"href"`
	Category    string             `json:"category"`
	ColumnCode  string             `json:"columnCode"`
	Cover       string             `json:"cover"`
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
	Name, Href, Image, ViewBox string
	Width, Height              int
}
type Page struct {
	ActiveNav                             string
	Title, Kind, BodyClass, RootPrefix    string
	ColumnCode                            string
	ColumnPaths                           template.JS
	Items                                 []*Article
	Article, PreviousArticle, NextArticle *Article
	Parent                                *Link
	Breadcrumbs                           []Link
	Side, Pages                           []Link
	Hero                                  []*Article
	Notices, Topics                       []Group
	Experts                               Group
	DataCenterURL                         string
	MemberLoginPath, MemberRegisterPath   string
	Branches                              []Branch
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
			seen[col.ID] = true
			if _, exists := c.byName[col.Name]; !exists {
				c.byName[col.Name] = col.ID
			}
			c.byCode[col.Code] = col.ID
			c.columns = append(c.columns, col)
			queue = append(queue, col.ID)
			_, items, err := g.source.FetchColumnArticles(ctx, col.ID)
			if err != nil {
				return nil, err
			}
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
				if cover == "" {
					cover = g.cfg.Site.FallbackCover
				}
				v := &Article{ID: a.ID, Title: a.Title, Summary: a.Summary, Category: col.Name, ColumnCode: col.Code, Source: a.Source, Date: a.PublishTime.Format("2006-01-02"), Href: href, Cover: cover, Video: a.Type == 2}
				if existing, ok := c.byID[a.ID]; ok {
					copy := *existing
					copy.Category, copy.ColumnCode = col.Name, col.Code
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
	// A home slot controls placement on the front page, not the detail page's
	// navigation. Prefer an actual public column when the article has one;
	// otherwise use the public section represented by that home slot.
	for _, article := range c.all {
		if !isHomeColumnCode(article.ColumnCode) {
			continue
		}
		preferred := homePublicColumnCode(g.cfg.Site.HeroColumn, article.ColumnCode)
		var selected model.Column
		for _, column := range c.columns {
			if isHomeColumnCode(column.Code) || isPrivateColumnCode(column.Code) {
				continue
			}
			for _, assigned := range c.groups[column.ID] {
				if assigned.ID != article.ID {
					continue
				}
				if selected.ID == 0 || column.Code == preferred {
					selected = column
				}
				break
			}
			if selected.Code == preferred {
				break
			}
		}
		if selected.ID == 0 {
			for _, column := range c.columns {
				if column.Code == preferred {
					selected = column
					break
				}
			}
		}
		if selected.ID == 0 {
			for _, column := range c.columns {
				if !isHomeColumnCode(column.Code) && !isPrivateColumnCode(column.Code) {
					selected = column
					break
				}
			}
		}
		if selected.ID != 0 {
			article.ColumnCode, article.Category = selected.Code, selected.Name
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

func isHomeColumnCode(code string) bool {
	return code == "home" || strings.HasPrefix(code, "home-")
}

func homePublicColumnCode(heroColumn, code string) string {
	switch code {
	case "home", "home-hero":
		if code == "home-hero" && heroColumn != "" {
			return heroColumn
		}
		return "news"
	case "home-experts":
		return "about-expert-insights"
	default:
		return strings.TrimPrefix(code, "home-")
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
	policy.AllowElements("video", "source")
	policy.AllowAttrs("src", "poster", "controls", "preload", "width", "height").OnElements("video")
	policy.AllowAttrs("src", "type").OnElements("source")
	policy.AllowRelativeURLs(true)
	policy.AllowURLSchemes("http", "https")
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	var visit func(*html.Node)
	visit = func(n *html.Node) {
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

func groupByID(c *catalog, id int64, limit int) Group {
	name := ""
	descendants := map[int64]bool{}
	for _, column := range c.columns {
		if column.ID == id {
			name = column.Name
		}
	}
	if id > 0 {
		descendants[id] = true
		for changed := true; changed; {
			changed = false
			for _, column := range c.columns {
				if !descendants[column.ID] && descendants[column.ParentID] && !isPrivateColumnCode(column.Code) {
					descendants[column.ID] = true
					changed = true
				}
			}
		}
	}
	articleIDs := map[int64]bool{}
	for columnID := range descendants {
		for _, article := range c.groups[columnID] {
			articleIDs[article.ID] = true
		}
	}
	items := make([]*Article, 0, len(articleIDs))
	for _, article := range c.all {
		if articleIDs[article.ID] {
			items = append(items, article)
		}
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	href := "news.html"
	if id > 0 {
		href = fmt.Sprintf("list/%d/1.html", id)
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

// Home slots have their own columns under camie-home. Older data sets do not
// have those columns yet, so their original public section remains a fallback.
func homeGroup(c *catalog, homeCode, fallbackCode string, limit int) Group {
	if _, exists := c.byCode[homeCode]; exists {
		return group(c, homeCode, limit)
	}
	return group(c, fallbackCode, limit)
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
	columns := make(map[int64]model.Column, len(c.columns))
	for _, column := range c.columns {
		columns[column.ID] = column
	}
	activeID := c.byCode[currentCode]
	if activeID == top.ID {
		activeID = 0
		for _, column := range c.columns {
			if column.ParentID == top.ID && !isPrivateColumnCode(column.Code) {
				activeID = column.ID
				break
			}
		}
	} else {
		for activeID != 0 && columns[activeID].ParentID != top.ID {
			activeID = columns[activeID].ParentID
		}
	}
	var links []Link
	for _, column := range c.columns {
		if column.ParentID != top.ID || isPrivateColumnCode(column.Code) {
			continue
		}
		active := column.ID == activeID
		link := Link{Name: column.Name, Href: groupByID(c, column.ID, 0).Href, Active: active}
		for _, child := range c.columns {
			if child.ParentID == column.ID && !isPrivateColumnCode(child.Code) {
				link.Children = append(link.Children, Link{Name: child.Name, Href: groupByID(c, child.ID, 0).Href, Active: child.Code == currentCode})
			}
		}
		links = append(links, link)
	}
	if len(links) == 0 {
		links = append(links, Link{Name: top.Name, Href: groupByID(c, top.ID, 0).Href, Active: true})
	}
	if top.Code == "videos" {
		links = append(links, Link{Name: "会员专享", Href: "pages/member.html?column=会员专享&mode=video"})
	}
	return links
}

func activeNav(c *catalog, p Page) string {
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
	c, err := g.load(ctx)
	if err != nil {
		return result, err
	}
	columnPaths := make(map[string][]string, len(c.columns))
	columnsByID := make(map[int64]model.Column, len(c.columns))
	for _, column := range c.columns {
		columnsByID[column.ID] = column
	}
	for _, column := range c.columns {
		if isPrivateColumnCode(column.Code) {
			continue
		}
		var path []string
		for current := column; current.ID != 0; current = columnsByID[current.ParentID] {
			path = append(path, current.Name)
		}
		for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
			path[left], path[right] = path[right], path[left]
		}
		columnPaths[column.Code] = path
	}
	columnPathJSON, err := json.Marshal(columnPaths)
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
		p.ColumnPaths = template.JS(columnPathJSON)
		currentColumn := p.ColumnCode
		if p.Article != nil {
			currentColumn = p.Article.ColumnCode
		}
		top := topColumn(c, currentColumn)
		if top.ID != 0 && top.Code != currentColumn {
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
				ancestors = append(ancestors, Link{Name: col.Name, Href: groupByID(c, col.ID, 0).Href})
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
		Hero:      homeGroup(c, "home-hero", g.cfg.Site.HeroColumn, 3).Items,
		Branches: []Branch{
			{Name: "水分会", Code: "branch-water", Image: "assets/images/branch-water.png"},
			{Name: "大气分会", Code: "branch-atmosphere", Image: "assets/images/branch-atmosphere.png"},
			{Name: "固废分会", Code: "branch-solid-waste", Image: "assets/images/branch-solid-waste.png"},
			{Name: "环境监测分会", Code: "branch-monitoring", Image: "assets/images/branch-monitoring.png"},
			{Name: "噪声分会", Code: "branch-noise", Image: "assets/images/branch-noise.png"},
			{Name: "紫外线分会", Code: "branch-uv", Image: "assets/images/branch-uv.png"},
			{Name: "臭氧分会", Code: "branch-ozone", Image: "assets/images/branch-odor.png"},
			{Name: "人工智能分会", Code: "branch-ai", Image: "assets/images/branch-ai.png"},
			{Name: "环境工程分会", Code: "branch-engineering", Image: "assets/images/branch-engineering.png"},
		},
		PartnerRows: [][]Partner{
			{
				{Name: "南京大学环境学院", Href: "https://www.nju.edu.cn/", Image: "assets/images/partner-logos/nanjing-university.png", ViewBox: "40 14 526 521", Width: 600, Height: 535},
				{Name: "紫金龙净环保新能源股份有限公司", Href: "https://www.longking.com.cn/", Image: "assets/images/partner-logos/longking.png", ViewBox: "35 35 280 282", Width: 356, Height: 349},
				{Name: "北京城市排水集团有限责任公司", Href: "https://www.bdc.cn/", Image: "assets/images/partner-logos/beijing-drainage.png", ViewBox: "0 78 438 125", Width: 438, Height: 258},
				{Name: "苏州帝瀚环保科技股份有限公司", Href: "https://www.dihillgreen.com/", Image: "assets/images/partner-logos/dihill.png", ViewBox: "85 385 809 162", Width: 945, Height: 944},
				{Name: "江苏一环集团有限公司", Href: "http://www.yihuan.com/", Image: "assets/images/partner-logos/jiangsu-yihuan.png", ViewBox: "0 40 1178 716", Width: 1178, Height: 784},
				{Name: "河南康宁特环保科技股份有限公司", Href: "https://www.knthb.com/", Image: "assets/images/partner-logos/kangningte.png", ViewBox: "0 0 167 63", Width: 167, Height: 63},
			},
			{
				{Name: "中国天楹股份有限公司", Href: "https://www.cnty.cn/", Image: "assets/images/partner-logos/cnty.png", ViewBox: "0 8 192 176", Width: 192, Height: 192},
				{Name: "杰瑞新能源再生循环科技有限公司", Href: "https://www.jereh.com/cn/", Image: "assets/images/partner-logos/jereh-recycling.png", ViewBox: "0 175 536 190", Width: 536, Height: 536},
				{Name: "合肥通用机械研究院有限公司", Href: "http://www.hgmri.com/", Image: "assets/images/partner-logos/hefei-general-machinery.png", ViewBox: "10 45 1045 376", Width: 1068, Height: 446},
				{Name: "长江生态环保集团有限公司", Href: "https://www.yeec.com.cn/", Image: "assets/images/partner-logos/yangtze-ecology.png", ViewBox: "0 0 418 55", Width: 418, Height: 55},
				{Name: "科林环保技术有限责任公司", Href: "https://www.kelin-china.com/", Image: "assets/images/partner-logos/kelin.png", ViewBox: "25 105 441 303", Width: 500, Height: 500},
				{Name: "中车产业投资有限公司", Href: "https://www.crrcgc.cc/cytz/277_19585/index.html", Image: "assets/images/partner-logos/crrc-investment.png", ViewBox: "20 112 560 208", Width: 600, Height: 434},
			},
		},
	}
	for _, n := range []string{"news-notice", "news-association", "news-member"} {
		home.Notices = append(home.Notices, homeGroup(c, "home-"+n, n, 6))
	}
	for _, topic := range []string{
		"training-meetings",
		"standards-work",
		"standards-innovation",
		"training-international",
		"training-talent",
		"policy-reports",
	} {
		home.Topics = append(home.Topics, homeGroup(c, "home-"+topic, topic, 6))
	}
	dataCenter := group(c, "policy-data", 1)
	home.DataCenterURL = dataCenter.Href
	if dataCenter.Feature != nil {
		home.DataCenterURL = dataCenter.Feature.Href
	}
	home.Experts = homeGroup(c, "home-experts", "about-expert-insights", 3)
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
		if isPrivateColumnCode(col.Code) {
			continue
		}
		if col.Name == "关于协会" || col.Name == "协会简介" {
			p := Page{Title: col.Name, ColumnCode: col.Code, Kind: "about", BodyClass: "about-page", Side: side(c, col.Code)}
			items := c.groups[col.ID]
			if col.Code == "about" && len(c.groups[c.byCode["about-introduction"]]) > 0 {
				items = c.groups[c.byCode["about-introduction"]]
			}
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
		alias := columnAlias(col.Code)
		if err = makeLists(col.Name, col.Code, fmt.Sprintf("list/%d", col.ID), alias, groupByID(c, col.ID, 0).Items); err != nil {
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
	for _, a := range c.all {
		if a.Video {
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
	for _, a := range c.all {
		if strings.HasPrefix(a.Href, "http") {
			continue
		}
		p := Page{Title: a.Title, ColumnCode: a.ColumnCode, Kind: "article", BodyClass: "article-page", Article: a, Side: side(c, a.ColumnCode), BackHref: group(c, a.ColumnCode, 0).Href}
		if a.Video {
			p.Kind = "video"
			p.BodyClass = "video-page"
			if firstVideo == nil {
				firstVideo = a
			}
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
	result.GeneratedAt = time.Now()
	return result, nil
}

func columnAlias(code string) string {
	aliases := map[string]string{
		"party": "pages/party.html", "ministry": "pages/ministry.html", "news": "pages/news.html",
		"training": "pages/training.html", "standards": "pages/standards.html", "about": "pages/about.html",
		"policy-research": "pages/policy.html", "about-experts": "pages/experts.html", "videos": "pages/videos.html",
	}
	return aliases[code]
}
