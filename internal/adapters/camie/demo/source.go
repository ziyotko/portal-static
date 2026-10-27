package demo

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"portal-static/internal/adapters/camie/model"
)

//go:embed english-about.html
var englishAboutContent string

// Source is a self-contained preview fixture matching the accepted 2026-09
// CAMIE column hierarchy. Member-only branches deliberately contain no body
// content because those pages are populated by authenticated APIs at runtime.
type Source struct {
	Columns []model.Column
	Items   map[int64][]model.Article
}

type columnSpec struct {
	Name     string
	Code     string
	Children []columnSpec
}

var navigation = []columnSpec{
	{Name: "党建专栏", Code: "party", Children: []columnSpec{{Name: "党建要闻", Code: "party-news"}, {Name: "学习教育", Code: "party-study"}, {Name: "支部活动", Code: "party-branch-activity"}}},
	{Name: "部委动态", Code: "ministry", Children: []columnSpec{{Name: "工作动态", Code: "ministry-work"}, {Name: "政策文件", Code: "ministry-policy-files", Children: []columnSpec{{Name: "国家鼓励发展的重大环保技术装备目录", Code: "ministry-major-equipment-catalogue"}, {Name: "环保装备制造业规范条件企业", Code: "ministry-qualified-enterprises"}, {Name: "重大环保技术装备创新任务揭榜挂帅", Code: "ministry-innovation-tasks"}, {Name: "其他文件", Code: "ministry-other-files"}}}, {Name: "政策解读", Code: "ministry-policy-interpretation"}}},
	{Name: "新闻中心", Code: "news", Children: []columnSpec{{Name: "热点关注", Code: "news-hot"}, {Name: "通知公告", Code: "news-notice"}, {Name: "协会动态", Code: "news-association"}, {Name: "会员动态", Code: "news-member"}}},
	{Name: "交流培训", Code: "training", Children: []columnSpec{{Name: "会议活动", Code: "training-meetings", Children: []columnSpec{{Name: "环保装备高质量发展大会", Code: "training-meetings-conference"}, {Name: "其他活动", Code: "training-meetings-other"}}}, {Name: "人才培训", Code: "training-talent", Children: []columnSpec{{Name: "通知及动态", Code: "training-talent-notices"}, {Name: "人才交流平台", Code: "training-talent-exchange"}}}, {Name: "国际交流与合作", Code: "training-international", Children: []columnSpec{{Name: "出海培训", Code: "training-overseas-training"}, {Name: "海外考察", Code: "training-overseas-visits"}, {Name: "交流对接", Code: "training-business-matching"}}}}},
	{Name: "科技标准", Code: "standards", Children: []columnSpec{{Name: "创新成果转化", Code: "standards-innovation", Children: []columnSpec{{Name: "科技成果评价", Code: "standards-achievement-evaluation"}, {Name: "科技成果转化平台", Code: "standards-transfer-platform"}}}, {Name: "标准工作", Code: "standards-work", Children: []columnSpec{{Name: "通知及动态", Code: "standards-work-notices"}, {Name: "协会标准发布", Code: "standards-publications"}}}, {Name: "绿色技术推广", Code: "standards-green-promotion"}}},
	{Name: "关于协会", Code: "about", Children: []columnSpec{{Name: "协会简介", Code: "about-introduction"}, {Name: "组织架构", Code: "about-organization"}, {Name: "协会章程", Code: "about-charter"}, {Name: "分支机构", Code: "about-branches", Children: []columnSpec{{Name: "水分会", Code: "branch-water"}, {Name: "大气分会", Code: "branch-atmosphere"}, {Name: "固废分会", Code: "branch-solid-waste"}, {Name: "环境监测分会", Code: "branch-monitoring"}, {Name: "噪声分会", Code: "branch-noise"}, {Name: "紫外线分会", Code: "branch-uv"}, {Name: "臭氧分会", Code: "branch-ozone"}, {Name: "人工智能分会", Code: "branch-ai"}, {Name: "环境工程分会", Code: "branch-engineering"}}}, {Name: "专家委员会", Code: "about-experts"}, {Name: "信息公开", Code: "about-disclosure"}}},
	{Name: "会员中心", Code: "member", Children: []columnSpec{{Name: "行业报告", Code: "member-reports"}, {Name: "数据中心", Code: "member-data"}, {Name: "电子刊物", Code: "member-publications"}, {Name: "我的空间", Code: "member-space"}}},
	{Name: "视频专区", Code: "videos", Children: []columnSpec{{Name: "视频资讯", Code: "videos-news"}, {Name: "会员专享", Code: "videos-members"}}},
	{Name: "政策研究", Code: "policy-research", Children: []columnSpec{{Name: "行业报告", Code: "policy-reports"}, {Name: "数据中心", Code: "policy-data"}, {Name: "国家鼓励发展的重大环保技术装备目录", Code: "policy-major-equipment-catalogue"}, {Name: "环保装备制造业规范条件企业", Code: "policy-qualified-enterprises"}, {Name: "重大环保技术装备创新任务揭榜挂帅", Code: "policy-innovation-tasks"}}},
}

// The homepage preview fills every notice tab with mixed title lengths so
// single-line truncation and the eleven-row layout can be checked visually.
var previewNoticeTitles = map[string][]string{
	"news-notice": {
		"【预览】关于召开环保装备行业交流会的通知",
		"【预览】人才培训班报名开启",
		"【预览】关于征集《工业锅炉烟气多污染物协同治理技术规范》团体标准意见的通知",
		"【预览】会员服务系统维护安排",
		"【预览】绿色低碳技术成果推荐工作有关事项的通知",
		"【预览】关于举办人工智能赋能环保行业高端人才培训班的通知",
		"【预览】节能装备专题研讨会日程公布",
		"【预览】关于开展环保装备企业需求调研及技术交流活动的通知",
	},
	"news-association": {
		"【预览】协会组织行业专家开展技术交流",
		"【预览】协会工作简讯",
		"【预览】环保装备制造业高质量发展座谈会围绕产业协同与创新应用开展交流",
		"【预览】协会走访会员单位了解发展需求",
		"【预览】绿色技术推广专题会议在京举行",
		"【预览】协会参与节能环保装备标准化工作研讨并征集企业建议",
		"【预览】分支机构年度工作交流会召开",
		"【预览】协会秘书处开展环保装备产业链供需对接调研",
	},
	"news-member": {
		"【预览】会员单位发布新型节能设备",
		"【预览】会员动态速览",
		"【预览】会员企业围绕工业废水治理与资源化利用开展联合技术攻关",
		"【预览】企业展示绿色制造实践成果",
		"【预览】会员单位参与国际环保技术交流",
		"【预览】多家会员企业分享智能化环保装备研发及应用经验",
		"【预览】环保装备项目完成阶段性验收",
		"【预览】会员单位举办大气污染治理技术应用和运维经验交流会",
	},
}

func NewSource() *Source {
	source := &Source{Items: make(map[int64][]model.Article)}
	nextColumnID := int64(1)
	var appendColumns func([]columnSpec, int64)
	appendColumns = func(specs []columnSpec, parentID int64) {
		for sortIndex, spec := range specs {
			id := nextColumnID
			nextColumnID++
			source.Columns = append(source.Columns, model.Column{ID: id, Name: spec.Name, Code: spec.Code, TemplateID: 1, TemplateName: "环保机械协会", ParentID: parentID, Sort: sortIndex})
			appendColumns(spec.Children, id)
		}
	}
	appendColumns(navigation, 0)
	appendColumns([]columnSpec{{Name: "首页内容", Code: "home", Children: []columnSpec{
		{Name: "轮播图", Code: "home-hero"},
		{Name: "分支机构", Code: "home-fenzhijigou", Children: []columnSpec{{Name: "水分会", Code: "main-branch-water"}, {Name: "大气分会", Code: "main-branch-atmosphere"}, {Name: "固废分会", Code: "main-branch-solid-waste"}, {Name: "环境监测分会", Code: "main-branch-monitoring"}, {Name: "噪声分会", Code: "main-branch-noise"}, {Name: "紫外线分会", Code: "main-branch-uv"}, {Name: "臭氧分会", Code: "main-branch-ozone"}, {Name: "人工智能分会", Code: "main-branch-ai"}, {Name: "环境工程分会", Code: "main-branch-engineering"}}},
		{Name: "专家视野", Code: "home-expert-insights"},
	}}}, 0)
	appendColumns([]columnSpec{{Name: "About CAMIE", Code: "about-english"}}, 0)

	base := time.Date(2026, 9, 29, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	for _, column := range source.Columns {
		if isMemberColumn(column.Code) || (len(childrenOf(source.Columns, column.ID)) > 0 && column.Code != "training-meetings") {
			continue
		}
		if column.Code == "about-english" {
			source.Items[column.ID] = []model.Article{{ID: column.ID*1000 + 1, ColumnID: column.ID, Type: model.ArticleTypeContent, Title: "About CAMIE", Content: englishAboutContent, PublishTime: base}}
			continue
		}
		for index := 0; index < 12; index++ {
			articleType := model.ArticleTypeContent
			cover := "assets/images/hero-building.png"
			if column.Code == "home-hero" {
				cover = "assets/images/hero-preview-museum.jpg"
			}
			title := fmt.Sprintf("%s：环保装备行业高质量发展动态 %d", column.Name, index+1)
			if titles := previewNoticeTitles[column.Code]; index < len(titles) {
				title = titles[index]
			}
			if column.Code == "videos-news" {
				articleType = model.ArticleTypeVideo
				cover = "assets/images/video-thumb.png"
			}
			id := column.ID*1000 + int64(index+1)
			source.Items[column.ID] = append(source.Items[column.ID], model.Article{
				ID: id, ColumnID: column.ID, Type: articleType,
				Title:   title,
				Summary: "关注环保装备产业发展，推动行业交流、科技创新与成果转化。",
				Content: "<p>本条为静态预览内容，正式发布时由门户内容库提供正文。</p><p>页面正文、栏目来源和面包屑均在生成阶段写入 HTML。</p>",
				Cover:   cover, Source: "中国环保机械行业协会", PublishTime: base.AddDate(0, 0, -index), IsTop: index == 0,
			})
		}
	}
	return source
}

func isMemberColumn(code string) bool {
	return code == "member" || strings.HasPrefix(code, "member-") || code == "videos-members"
}

func childrenOf(columns []model.Column, parentID int64) []model.Column {
	var result []model.Column
	for _, column := range columns {
		if column.ParentID == parentID {
			result = append(result, column)
		}
	}
	return result
}

func (s *Source) FetchPageColumns(ctx context.Context, _ string, parent int64) ([]model.Column, error) {
	return childrenOf(s.Columns, parent), ctx.Err()
}

func (s *Source) FetchColumnArticles(ctx context.Context, id int64) (model.Column, []model.Article, error) {
	for _, column := range s.Columns {
		if column.ID == id {
			return column, append([]model.Article(nil), s.Items[id]...), ctx.Err()
		}
	}
	return model.Column{}, nil, fmt.Errorf("unknown column %d", id)
}

func (s *Source) FetchAttachments(ctx context.Context, _ int64) ([]model.Attachment, error) {
	return nil, ctx.Err()
}
