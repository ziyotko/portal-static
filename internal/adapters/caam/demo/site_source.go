package demo

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
	"portal-static/internal/adapters/caam/repository"
)

// SiteSource expands the homepage demo feed into a deterministic in-memory
// CMS. It implements the same repository contracts as the production source,
// allowing preview to exercise the complete site generator without a database.
type SiteSource struct {
	*Source
	columns            []model.Column
	columnsByID        map[int64]model.Column
	columnsByName      map[string]model.Column
	columnIDBySlotKey  map[string]int64
	articlesByID       map[int64]model.Article
	articlesByColumnID map[int64][]model.Article
	columnIDsByArticle map[int64][]int64
	mappings           []model.ArticleColumnMapping
}

func NewSiteSource(cfg config.Config) *SiteSource {
	source := &SiteSource{
		Source:             NewSource(),
		columnsByID:        make(map[int64]model.Column),
		columnsByName:      make(map[string]model.Column),
		columnIDBySlotKey:  make(map[string]int64),
		articlesByID:       make(map[int64]model.Article),
		articlesByColumnID: make(map[int64][]model.Article),
		columnIDsByArticle: make(map[int64][]int64),
	}
	nextColumnID := int64(2001)
	nextArticleID := int64(100001)
	nextMappingID := int64(1)

	addColumn := func(name string) model.Column {
		name = strings.TrimSpace(name)
		if existing, ok := source.columnsByName[name]; ok {
			return existing
		}
		column := model.Column{ID: nextColumnID, Name: name}
		nextColumnID++
		source.columns = append(source.columns, column)
		source.columnsByID[column.ID] = column
		source.columnsByName[name] = column
		return column
	}
	addArticle := func(column model.Column, article model.Article) {
		article.ID = strconv.FormatInt(nextArticleID, 10)
		article.ColumnID = column.ID
		if article.Type == 0 {
			article.Type = model.ArticleTypeContent
		}
		if article.PublishTime.IsZero() {
			article.PublishTime = time.Date(2026, 7, 16, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
		}
		source.articlesByID[nextArticleID] = article
		source.articlesByColumnID[column.ID] = append(source.articlesByColumnID[column.ID], article)
		source.columnIDsByArticle[nextArticleID] = []int64{column.ID}
		source.mappings = append(source.mappings, model.ArticleColumnMapping{
			ID: nextMappingID, ColumnID: column.ID, ArticleID: nextArticleID,
		})
		nextArticleID++
		nextMappingID++
	}

	for _, slot := range cfg.ContentSlots() {
		column := addColumn(slot.Name)
		source.columnIDBySlotKey[slot.Key] = column.ID
		for _, article := range source.Source.articles[slot.Key] {
			addArticle(column, article)
		}
	}

	sectionNames := caamSectionColumnNames(cfg)
	for index, name := range sectionNames {
		column := addColumn(name)
		if len(source.articlesByColumnID[column.ID]) > 0 {
			continue
		}
		if isChartColumn(cfg, name) {
			for month := 1; month <= 6; month++ {
				article := monthlyArticle(name, 2026, month, float64(80+index*3+month))
				article.PublishTime = time.Date(2026, time.Month(month), 20, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
				addArticle(column, article)
			}
			continue
		}
		for item := 1; item <= 2; item++ {
			addArticle(column, model.Article{
				Type: model.ArticleTypeContent, Title: name + "演示内容" + strconv.Itoa(item),
				Summary: "用于完整站点预览的演示内容。",
				Content: "<p>这是“" + name + "”的演示内容，用于验证页面、列表和文章静态化。</p>",
				Cover:   "assets/images/content-placeholder.png", Source: "中国汽车工业协会",
				PublishTime: time.Date(2026, 7, 16-item, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
			})
		}
	}
	return source
}

func caamSectionColumnNames(cfg config.Config) []string {
	a := cfg.About
	w := cfg.WorkPage
	s := cfg.StatsPage
	m := cfg.MembersPage
	p := cfg.PartyPage
	return []string{
		a.Intro, a.Leadership, a.Charter, a.Organization, a.Responsibilities, a.Honors,
		a.RotatingPresident, a.VicePresident, a.ExecutiveDirector, a.Director, a.MemberDelegate, a.RegularMember,
		w.Headline, w.Association, w.Branch, w.Industry, w.International, w.Expo, w.Platform,
		s.DomesticReports, s.OverseasReports, s.ProductionReports, s.ImportExportReports,
		s.DomesticChart, s.OverseasChart, s.ProductionChart, s.ImportExportChart,
		m.Work, m.Style, m.Policy, m.Charter, m.Fees, m.President, m.VicePresident,
		m.BranchIntro, m.BranchRules, m.BranchService, m.BranchRoster, m.Management,
		m.ExecutiveDirector, m.Director, m.Delegate, m.Member,
		p.Work, p.Study, p.News, p.Carousel,
	}
}

func isChartColumn(cfg config.Config, name string) bool {
	name = strings.TrimSpace(name)
	return name == strings.TrimSpace(cfg.StatsPage.DomesticChart) ||
		name == strings.TrimSpace(cfg.StatsPage.OverseasChart) ||
		name == strings.TrimSpace(cfg.StatsPage.ProductionChart) ||
		name == strings.TrimSpace(cfg.StatsPage.ImportExportChart)
}

func (s *SiteSource) ResolveColumnID(_ context.Context, slot config.SlotConfig) (int64, error) {
	if id := s.columnIDBySlotKey[slot.Key]; id > 0 {
		return id, nil
	}
	return s.ResolveGlobalColumnID(context.Background(), slot.Name)
}

func (s *SiteSource) ResolveGlobalColumnID(_ context.Context, name string) (int64, error) {
	column, ok := s.columnsByName[strings.TrimSpace(name)]
	if !ok {
		return 0, repository.ErrColumnNotFound
	}
	return column.ID, nil
}

func (s *SiteSource) FetchByColumn(_ context.Context, slot config.SlotConfig) ([]model.Article, error) {
	id, err := s.ResolveColumnID(context.Background(), slot)
	if err != nil {
		return nil, err
	}
	return limitedCopy(s.articlesByColumnID[id], slot.Limit), nil
}

func (s *SiteSource) FetchColumns(context.Context) ([]model.Column, error) {
	return append([]model.Column(nil), s.columns...), nil
}

func (s *SiteSource) FetchAllArticles(context.Context) ([]model.Article, error) {
	ids := s.sortedArticleIDs()
	return s.articlesForIDs(ids), nil
}

func (s *SiteSource) FetchArticleColumnMappingsBatch(_ context.Context, afterID int64, limit int) ([]model.ArticleColumnMapping, error) {
	result := make([]model.ArticleColumnMapping, 0, limit)
	for _, mapping := range s.mappings {
		if mapping.ID <= afterID {
			continue
		}
		result = append(result, mapping)
		if limit > 0 && len(result) == limit {
			break
		}
	}
	return result, nil
}

func (s *SiteSource) FetchListArticlesByIDs(_ context.Context, ids []int64) ([]model.Article, error) {
	return s.articlesForIDs(ids), nil
}

func (s *SiteSource) FetchDetailArticlesByIDs(_ context.Context, ids []int64) ([]model.Article, error) {
	return s.articlesForIDs(ids), nil
}

func (s *SiteSource) FetchArticleColumns(_ context.Context, articleID int64) ([]model.Column, error) {
	return s.columnsForArticle(articleID)
}

func (s *SiteSource) FetchArticleRelatedColumns(_ context.Context, articleID int64) ([]model.Column, error) {
	return s.columnsForArticle(articleID)
}

func (s *SiteSource) FetchColumnArticles(_ context.Context, columnID int64) (model.Column, []model.Article, error) {
	column, ok := s.columnsByID[columnID]
	if !ok {
		return model.Column{}, nil, repository.ErrColumnNotFound
	}
	return column, append([]model.Article(nil), s.articlesByColumnID[columnID]...), nil
}

func (s *SiteSource) FetchArticle(_ context.Context, articleID int64) (model.Column, model.Article, error) {
	article, ok := s.articlesByID[articleID]
	if !ok {
		return model.Column{}, model.Article{}, repository.ErrArticleNotPublished
	}
	column := s.columnsByID[article.ColumnID]
	return column, article, nil
}

func (s *SiteSource) FetchPublishedByColumnName(ctx context.Context, name string) (model.Column, []model.Article, error) {
	id, err := s.ResolveGlobalColumnID(ctx, name)
	if err != nil {
		return model.Column{}, nil, err
	}
	return s.FetchPublishedByColumnID(ctx, id)
}

func (s *SiteSource) FetchPublishedByColumnNameLimit(ctx context.Context, name string, types []int, limit int) (model.Column, []model.Article, error) {
	column, articles, err := s.FetchPublishedByColumnName(ctx, name)
	if err != nil {
		return model.Column{}, nil, err
	}
	allowed := make(map[int]struct{}, len(types))
	for _, articleType := range types {
		allowed[articleType] = struct{}{}
	}
	filtered := make([]model.Article, 0, len(articles))
	for _, article := range articles {
		if len(allowed) > 0 {
			if _, ok := allowed[article.Type]; !ok {
				continue
			}
		}
		filtered = append(filtered, article)
		if limit > 0 && len(filtered) == limit {
			break
		}
	}
	return column, filtered, nil
}

func (s *SiteSource) FetchPublishedByColumnID(_ context.Context, id int64) (model.Column, []model.Article, error) {
	column, ok := s.columnsByID[id]
	if !ok {
		return model.Column{}, nil, repository.ErrColumnNotFound
	}
	return column, append([]model.Article(nil), s.articlesByColumnID[id]...), nil
}

func (s *SiteSource) sortedArticleIDs() []int64 {
	ids := make([]int64, 0, len(s.articlesByID))
	for id := range s.articlesByID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (s *SiteSource) articlesForIDs(ids []int64) []model.Article {
	result := make([]model.Article, 0, len(ids))
	for _, id := range ids {
		if article, ok := s.articlesByID[id]; ok {
			result = append(result, article)
		}
	}
	return result
}

func (s *SiteSource) columnsForArticle(articleID int64) ([]model.Column, error) {
	ids, ok := s.columnIDsByArticle[articleID]
	if !ok {
		return nil, repository.ErrArticleNotPublished
	}
	result := make([]model.Column, 0, len(ids))
	for _, id := range ids {
		if column, exists := s.columnsByID[id]; exists {
			result = append(result, column)
		}
	}
	return result, nil
}
