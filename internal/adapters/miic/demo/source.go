package demo

import (
	"context"
	"sort"
	"strings"
	"time"

	"portal-static/internal/adapters/miic/model"
	"portal-static/internal/adapters/miic/repository"
)

type Source struct {
	columns  []model.Column
	articles map[int64][]model.Article
}

func NewSource() *Source {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	columns := []model.Column{
		{ID: 11, Name: "中心重要动态", Code: "important", TemplateID: 1, TemplateName: "资讯动态"},
		{ID: 20, Name: "最新", Code: "latest", TemplateID: 1, TemplateName: "资讯动态", Sort: 10},
		{ID: 21, Name: "中心动态", Code: "center", TemplateID: 1, TemplateName: "资讯动态", Sort: 20},
		{ID: 22, Name: "行业动态", Code: "industry", TemplateID: 1, TemplateName: "资讯动态", Sort: 30},
		{ID: 23, Name: "行业资讯", Code: "information", TemplateID: 1, TemplateName: "资讯动态", Sort: 40},
		{ID: 24, Name: "成果发布", Code: "results", TemplateID: 1, TemplateName: "资讯动态", Sort: 50},
		{ID: 31, Name: "信息技术服务", Code: "it-services", TemplateID: 2, TemplateName: "核心业务", Sort: 10},
		{ID: 32, Name: "软件产品及定制开发服务", Code: "software", TemplateID: 2, TemplateName: "核心业务", ParentID: 31, Sort: 10},
		{ID: 41, Name: "招聘信息", Code: "recruitment", TemplateID: 4, TemplateName: "关于我们", Sort: 10},
		{ID: 42, Name: "信息公开", Code: "disclosure", TemplateID: 4, TemplateName: "关于我们", Sort: 20},
	}
	makeArticle := func(id, column int64, day int, title, summary, cover string) model.Article {
		return model.Article{ID: id, ColumnID: column, Type: 1, Title: title, Summary: summary,
			Content: "<h2>内容摘要</h2><p>本页面由 MIIC 静态化服务演示数据生成，用于验证模板、链接和发布流程。</p>",
			Cover:   cover, Source: "机械工业信息中心", PublishTime: time.Date(2026, 8, day, 9, 0, 0, 0, loc)}
	}
	a1 := makeArticle(1001, 21, 18, "聚焦行业数智化转型，构建机械工业高质量发展新动能", "围绕数据价值释放、转型能力提升和行业共性需求，持续完善数智化赋能服务体系。", "assets/unsplash-data-dashboard.jpg")
	a2 := makeArticle(1002, 22, 11, "机械工业运行总体平稳，产业结构持续优化升级", "跟踪行业运行态势与重点领域变化，为产业分析与科学决策提供专业信息参考。", "assets/unsplash-gears.jpg")
	a3 := makeArticle(1003, 23, 8, "制造业数字化转型加速向全链条协同纵深推进", "从单点技术应用走向流程重构与数据协同，数智化正在形成产业发展新优势。", "assets/unsplash-appliance-line.jpg")
	a4 := makeArticle(1004, 24, 3, "机械工业数字化转型路径与实践研究成果发布", "总结典型实践，提炼转型方法，为行业企业数字化建设提供可复用的经验参考。", "assets/unsplash-precision-lab.jpg")
	service := makeArticle(2001, 32, 20, "软件产品及定制开发服务", "面向机关单位与行业协会提供标准化软件产品、定制开发及实施服务。", "assets/unsplash-software-development.jpg")
	service.Content = "<h2>服务内容</h2><p>后台可维护的服务详情演示内容。</p><h2>服务流程</h2><ol><li>需求沟通</li><li>方案设计</li><li>开发实施</li></ol>"
	recruitment := makeArticle(3001, 41, 19, "前端工程师", "招聘状态以后台发布信息为准。", "")
	recruitment.Content = "<h2>岗位职责</h2><p>负责门户及业务系统前端开发。</p>"
	disclosure := makeArticle(3002, 42, 18, "2026年机械工业信息中心部门预算", "部门预算公开文件。", "")
	disclosure.URL = "https://www.miic.com.cn/api/v1/file/info?filePath=inforurl/202604/01f84cf78400200e.pdf"
	important := []model.Article{a1, a2, a4}
	for i := range important {
		important[i].ColumnID = 11
	}
	return &Source{columns: columns, articles: map[int64][]model.Article{
		11: important, 20: {a1, a2, a3, a4}, 21: {a1}, 22: {a2}, 23: {a3}, 24: {a4},
		32: {service}, 41: {recruitment}, 42: {disclosure},
	}}
}

func (s *Source) FetchByColumnName(_ context.Context, name string, limit int) (model.Column, []model.Article, error) {
	for _, column := range s.columns {
		if column.Name == name {
			items := append([]model.Article(nil), s.articles[column.ID]...)
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}
			return column, items, nil
		}
	}
	return model.Column{}, nil, repository.ErrColumnNotFound
}

func (s *Source) ResolveGlobalColumnID(_ context.Context, name string) (int64, error) {
	for _, column := range s.columns {
		if column.Name == strings.TrimSpace(name) {
			return column.ID, nil
		}
	}
	return 0, repository.ErrColumnNotFound
}

func (s *Source) FetchPageColumns(_ context.Context, pageName string, parentID int64) ([]model.Column, error) {
	templateIDs := map[string]int64{"资讯动态": 1, "核心业务": 2, "服务平台": 3, "关于我们": 4}
	templateID, ok := templateIDs[pageName]
	if !ok {
		return nil, repository.ErrTemplateNotFound
	}
	var result []model.Column
	for _, column := range s.columns {
		if column.TemplateID == templateID && column.ParentID == parentID {
			result = append(result, column)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Sort != result[j].Sort {
			return result[i].Sort < result[j].Sort
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func (s *Source) FetchPageColumnArticles(ctx context.Context, pageName, columnName string, limit int) (model.Column, []model.Article, error) {
	columns, err := s.FetchPageColumns(ctx, pageName, 0)
	if err != nil {
		return model.Column{}, nil, err
	}
	for _, column := range columns {
		if column.Name == columnName {
			items := append([]model.Article(nil), s.articles[column.ID]...)
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}
			return column, items, nil
		}
	}
	return model.Column{}, nil, repository.ErrColumnNotFound
}

func (*Source) FetchAttachments(context.Context, int64) ([]model.Attachment, error) { return nil, nil }
func (s *Source) FetchColumns(context.Context) ([]model.Column, error) {
	return append([]model.Column(nil), s.columns...), nil
}

func (s *Source) FetchColumnArticles(_ context.Context, id int64) (model.Column, []model.Article, error) {
	for _, column := range s.columns {
		if column.ID == id {
			return column, append([]model.Article(nil), s.articles[id]...), nil
		}
	}
	return model.Column{}, nil, repository.ErrColumnNotFound
}

func (s *Source) FetchArticle(_ context.Context, id int64) (model.Column, model.Article, error) {
	for _, column := range s.columns {
		for _, article := range s.articles[column.ID] {
			if article.ID == id {
				return column, article, nil
			}
		}
	}
	return model.Column{}, model.Article{}, repository.ErrArticleNotPublished
}

func (s *Source) FetchArticleColumns(_ context.Context, id int64) ([]model.Column, error) {
	var result []model.Column
	for _, column := range s.columns {
		for _, article := range s.articles[column.ID] {
			if article.ID == id && model.HasStaticDetail(article.Type) {
				result = append(result, column)
				break
			}
		}
	}
	if len(result) == 0 {
		return nil, repository.ErrArticleNotPublished
	}
	return result, nil
}

func (s *Source) FetchArticleRelatedColumns(ctx context.Context, id int64) ([]model.Column, error) {
	columns, err := s.FetchArticleColumns(ctx, id)
	if errorsIsArticleMissing(err) {
		return nil, nil
	}
	return columns, err
}

func errorsIsArticleMissing(err error) bool { return err == repository.ErrArticleNotPublished }

func (s *Source) FetchAllArticles(context.Context) ([]model.Article, error) {
	seen := map[int64]bool{}
	var result []model.Article
	for _, items := range s.articles {
		for _, article := range items {
			if !seen[article.ID] {
				seen[article.ID] = true
				result = append(result, article)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}
