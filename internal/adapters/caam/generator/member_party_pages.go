package generator

import (
	"context"
	"html"
	"html/template"
	"math"
	"strings"
	"time"
)

type MemberSectionView struct {
	Key         string
	Title       string
	Description string
	Item        *ArticleView
}

type MemberRosterView struct {
	Key    string
	Title  string
	Active bool
	Items  []ArticleView
}

type MembersPageData struct {
	GeneratedAt    string
	Work           []ArticleView
	Style          []ArticleView
	WorkListHref   string
	StyleListHref  string
	NewsCover      string
	Management     template.HTML
	ManagementLink *ArticleView
	MoreSections   []MemberSectionView
	Branches       []MemberSectionView
	Presidents     []ArticleView
	VicePresidents []ArticleView
	MemberLogos    []ArticleView
	Rosters        []MemberRosterView
	FooterLinks    []LinkGroupView
}

type PartyPageData struct {
	GeneratedAt   string
	Headline      *ArticleView
	Carousel      []ArticleView
	Work          []ArticleView
	Study         []ArticleView
	WorkListHref  string
	StudyListHref string
	FooterLinks   []LinkGroupView
}

func (g *SiteGenerator) loadMembersColumns(ctx context.Context) ([]PreparedColumn, error) {
	m := g.cfg.MembersPage
	return g.loadNamedColumns(ctx, []namedColumnSpec{
		{key: "work", title: "会员工作", name: m.Work, types: []int{1, 2}, limit: 5},
		{key: "style", title: "会员风采", name: m.Style, types: []int{1, 2}, limit: 5},
		{key: "policy", title: "相关制度", name: m.Policy},
		{key: "charter", title: "协会章程", name: m.Charter},
		{key: "fees", title: "会费缴纳标准与办法", name: m.Fees},
		{key: "president", title: "会长单位", name: m.President},
		{key: "vice-president", title: "副会长单位", name: m.VicePresident},
		{key: "branch-intro", title: "机构介绍", name: m.BranchIntro},
		{key: "branch-rules", title: "管理办法", name: m.BranchRules},
		{key: "branch-service", title: "分支机构服务", name: m.BranchService},
		{key: "branch-roster", title: "机构名单", name: m.BranchRoster},
		{key: "management", title: "会员管理", name: m.Management},
		{key: "executive-director", title: "常务理事", name: m.ExecutiveDirector},
		{key: "director", title: "理事单位", name: m.Director},
		{key: "delegate", title: "会员代表", name: m.Delegate},
		{key: "member", title: "会员单位", name: m.Member},
	})
}

func (g *SiteGenerator) loadPartyColumns(ctx context.Context) ([]PreparedColumn, error) {
	p := g.cfg.PartyPage
	return g.loadNamedColumns(ctx, []namedColumnSpec{
		{key: "work", title: "工作动态", name: p.Work},
		{key: "study", title: "学习教育", name: p.Study},
		{key: "news", title: "党建要闻", name: p.News},
		{key: "carousel", title: "党建轮播", name: p.Carousel},
	})
}

func firstArticleView(items []ArticleView) *ArticleView {
	if len(items) == 0 {
		return nil
	}
	item := items[0]
	return &item
}

func (g *SiteGenerator) firstColumnContent(column PreparedColumn) template.HTML {
	if len(column.Articles) == 0 {
		return template.HTML("<p>暂无内容</p>")
	}
	article := column.Articles[0]
	if strings.TrimSpace(article.Content) != "" {
		return g.pages.sanitizeContent(article.Content)
	}
	if strings.TrimSpace(article.Summary) != "" {
		return template.HTML("<p>" + html.EscapeString(strings.TrimSpace(article.Summary)) + "</p>")
	}
	return template.HTML("<p>暂无内容</p>")
}

func (g *SiteGenerator) generateMembers(ctx context.Context) (StaticPageResult, error) {
	started := time.Now()
	if err := g.prepareDist(); err != nil {
		return StaticPageResult{}, err
	}
	columns, err := g.loadMembersColumns(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	footerLinks, err := g.pages.fetchFooterLinks(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	byKey := make(map[string]PreparedColumn, len(columns))
	total := 0
	for _, column := range columns {
		byKey[column.Key] = column
		total += len(column.Articles)
	}
	fallback := "assets/images/content-placeholder.png"
	views := func(key string) []ArticleView { return g.pageViews(byKey[key], "members", key, fallback) }
	work := views("work")
	newsCover := fallback
	if len(work) > 0 && work[0].Cover != "" {
		newsCover = work[0].Cover
	}
	makeSection := func(key, title, description string) MemberSectionView {
		return MemberSectionView{Key: key, Title: title, Description: description, Item: firstArticleView(views(key))}
	}
	data := MembersPageData{
		GeneratedAt: time.Now().In(g.pages.location).Format(time.RFC3339),
		Work:        work, Style: views("style"), WorkListHref: columnListHref(byKey["work"]), StyleListHref: columnListHref(byKey["style"]), NewsCover: newsCover,
		Management: g.firstColumnContent(byKey["management"]), ManagementLink: firstArticleView(views("management")),
		MoreSections: []MemberSectionView{
			makeSection("fees", "会费缴纳标准与办法", "加入协会的会费缴纳和联系方式"),
			makeSection("charter", "协会章程", "了解中国汽车工业协会章程"),
			makeSection("policy", "相关制度", "了解协会各项管理制度"),
		},
		Branches: []MemberSectionView{
			makeSection("branch-intro", "机构介绍", "了解分支机构设置、工作职责与专业领域。"), makeSection("branch-rules", "管理办法", "查阅分支机构设立、运行和监督管理相关制度。"),
			makeSection("branch-service", "分支机构服务", "获取行业研究、交流协作与专业服务信息。"), makeSection("branch-roster", "机构名单", "查询各专业分支机构及相关组织信息。"),
		},
		Presidents: views("president"), VicePresidents: views("vice-president"), MemberLogos: views("member"),
		Rosters: []MemberRosterView{
			{Key: "executive-director", Title: "常务理事", Active: true, Items: views("executive-director")},
			{Key: "director", Title: "理事单位", Items: views("director")},
			{Key: "delegate", Title: "会员代表", Items: views("delegate")},
			{Key: "member", Title: "会员单位", Items: views("member")},
		},
		FooterLinks: footerLinks,
	}
	output, err := g.renderSectionPage(g.cfg.MembersPage.Template, g.cfg.MembersPage.Output, data, "class=\"members-main\"")
	if err != nil {
		return StaticPageResult{}, err
	}
	result := StaticPageResult{GeneratedAt: time.Now().In(g.pages.location), DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000, Page: "members", TotalItems: total, Output: output, Gray: grayCode(g.grayscale)}
	g.logger.Info("会员专区静态页生成成功", "items", total, "output", output)
	return result, nil
}

func (g *SiteGenerator) generateParty(ctx context.Context) (StaticPageResult, error) {
	started := time.Now()
	if err := g.prepareDist(); err != nil {
		return StaticPageResult{}, err
	}
	columns, err := g.loadPartyColumns(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	footerLinks, err := g.pages.fetchFooterLinks(ctx)
	if err != nil {
		return StaticPageResult{}, err
	}
	byKey := make(map[string]PreparedColumn, len(columns))
	total := 0
	for _, column := range columns {
		byKey[column.Key] = column
		total += len(column.Articles)
	}
	fallback := "assets/images/content-placeholder.png"
	news := g.pageViews(byKey["news"], "party", "news", fallback)
	data := PartyPageData{
		GeneratedAt: time.Now().In(g.pages.location).Format(time.RFC3339),
		Headline:    firstArticleView(news), Carousel: g.pageViews(byKey["carousel"], "party", "carousel", fallback),
		Work:          g.pageViews(byKey["work"], "party", "work", fallback),
		Study:         g.pageViews(byKey["study"], "party", "study", fallback),
		WorkListHref:  columnListHref(byKey["work"]),
		StudyListHref: columnListHref(byKey["study"]),
		FooterLinks:   footerLinks,
	}
	output, err := g.renderSectionPage(g.cfg.PartyPage.Template, g.cfg.PartyPage.Output, data, "class=\"party-main\"")
	if err != nil {
		return StaticPageResult{}, err
	}
	result := StaticPageResult{GeneratedAt: time.Now().In(g.pages.location), DurationSeconds: math.Round(time.Since(started).Seconds()*1000) / 1000, Page: "party", TotalItems: total, Output: output, Gray: grayCode(g.grayscale)}
	g.logger.Info("党建专区静态页生成成功", "items", total, "output", output)
	return result, nil
}
