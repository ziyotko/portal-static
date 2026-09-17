// Package demo provides a database-free content source for the preserved
// homepage design preview. It intentionally shares the production template
// and generator so layout and interaction changes stay in sync.
package demo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"portal-static/internal/adapters/caam/config"
	"portal-static/internal/adapters/caam/model"
)

type Source struct {
	articles map[string][]model.Article
	links    map[string][]model.Article
}

type item struct {
	title   string
	summary string
	cover   string
	url     string
}

func NewSource() *Source {
	base := time.Date(2026, 7, 16, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	source := &Source{
		articles: make(map[string][]model.Article),
		links:    make(map[string][]model.Article),
	}

	source.articles["headline"] = makeArticles("headline", base, "assets/slice/hero-news.png", []item{
		{title: "2026年汽车工业统计工作会议在杭州召开", summary: "聚焦行业统计体系建设与数据服务能力提升。"},
	})
	source.articles["carousel"] = makeArticles("carousel", base, "assets/slice/hero-news.png", []item{
		{title: "“驭电出海 港通全球”，2026香港车博会新闻发布会在港成功召开", summary: "汇聚汽车产业力量，共话全球化发展新机遇。", cover: "assets/slice/hero-news.png"},
		{title: "中国汽车工业协会新能源汽车电池分会正式成立", summary: "搭建产业协同平台，推动动力电池高质量发展。", cover: "assets/images/banner-release.jpg"},
		{title: "绿智融合谋新局，2026商用车产业发展会议在十堰召开", summary: "围绕绿色化、智能化和国际化开展深入交流。", cover: "assets/images/banner-forum-2023.png"},
		{title: "2026中国汽车T10特别峰会在北京举行", summary: "行业代表共商汽车产业转型升级与创新路径。", cover: "assets/images/banner-summit-2023.png"},
		{title: "汽车产业创新技术论坛聚焦智能网联新趋势", summary: "探讨前沿技术应用，加快产业生态协同创新。", cover: "assets/images/banner-tech-forum.jpg"},
	})

	topNews := map[string][]item{
		"industry-news": {
			{title: "2026年新能源汽车下乡活动在两地同步启动"},
			{title: "汽车产业加快形成绿色低碳发展新格局"},
			{title: "我国汽车出口市场延续稳健增长态势"},
			{title: "智能网联汽车规模化应用进入新阶段"},
			{title: "供应链协同创新为产业升级注入新动能"},
		},
		"association-activity": {
			{title: "《中国汽车电驱动产业发展研究报告（2026版）》启动会在京召开"},
			{title: "中国汽车工业协会会员服务交流会成功举办"},
			{title: "汽车产业人才培养专题研讨会在芜湖召开"},
			{title: "2026中国汽车行业可持续发展实践案例启动征集"},
			{title: "汽车零部件产业协同发展座谈会顺利举行"},
		},
		"notice": {
			{title: "关于征求2026年新能源汽车下乡活动推荐车型意见的通知"},
			{title: "关于开展汽车行业数字化转型案例征集的通知"},
			{title: "2026中国汽车论坛参会报名正式开启"},
			{title: "关于组织推荐汽车供应链创新成果的函"},
			{title: "中国汽车工业协会人才招聘启事"},
		},
	}
	for key, items := range topNews {
		source.articles[key] = makeArticles(key, base.Add(-24*time.Hour), "", items)
	}

	work := map[string][]item{
		"work-file": {
			{title: "《2025中国汽车后市场年度发展报告》正式发布", cover: "assets/slice/work-photo.png"},
			{title: "中国汽车工业协会关于汽车行业绿色发展的倡议"},
			{title: "关于开展会员企业服务需求调研的通知"},
			{title: "汽车行业数字化转型优秀案例征集工作启动"},
			{title: "组织推荐2026中国汽车供应链创新成果的函"},
			{title: "关于加强行业自律与品牌建设的倡议书"},
		},
		"work-industry": {
			{title: "汽车产业运行总体平稳，新动能持续增强", cover: "assets/images/news-main.png"},
			{title: "新能源与智能网联融合发展专题研究启动"},
			{title: "汽车后市场服务体系建设研讨会召开"},
			{title: "商用车产业转型升级交流活动成功举办"},
			{title: "中国品牌汽车高质量发展调研工作启动"},
			{title: "汽车产业链韧性与安全水平持续提升"},
		},
		"work-smart": {
			{title: "智能网联汽车准入和上路通行试点稳步推进", cover: "assets/images/banner-tech-forum.jpg"},
			{title: "车路云一体化应用场景加速落地"},
			{title: "汽车数据安全与合规治理专题会议召开"},
			{title: "自动驾驶技术创新与产业化研讨会举办"},
			{title: "智能座舱生态合作交流活动在沪举行"},
			{title: "行业共议汽车软件标准体系建设"},
		},
		"work-brand": {
			{title: "中国汽车品牌向上发展专项行动正式启动", cover: "assets/images/topic-2.png"},
			{title: "会员企业品牌建设经验交流会成功举办"},
			{title: "汽车品牌国际传播能力建设研讨会召开"},
			{title: "行业品牌评价与服务体系持续完善"},
			{title: "汽车企业社会责任优秀实践案例发布"},
			{title: "中国品牌汽车海外发展专题调研启动"},
		},
		"work-expo": {
			{title: "2026中国汽车供应链大会筹备工作全面启动", cover: "assets/images/banner-forum-2023.png"},
			{title: "香港国际汽车及供应链博览会即将举办"},
			{title: "中国汽车论坛专题展区开放报名"},
			{title: "新能源汽车产业链展览展示活动启动招展"},
			{title: "汽车零部件创新成果展在上海举行"},
			{title: "国际汽车技术交流周日程正式发布"},
		},
		"work-standard": {
			{title: "汽车行业标准体系建设工作会议召开", cover: "assets/images/banner-release.jpg"},
			{title: "公开征求智能网联汽车相关标准意见"},
			{title: "新能源汽车安全技术标准研讨会举办"},
			{title: "汽车碳足迹核算标准研究取得阶段性成果"},
			{title: "车用动力电池回收利用标准持续完善"},
			{title: "道路车辆信息安全标准宣贯活动举行"},
		},
	}
	for key, items := range work {
		source.articles[key] = makeArticles(key, base.Add(-48*time.Hour), "assets/slice/work-photo.png", items)
	}

	industry := map[string][]item{
		"company-news": {
			{title: "中国汽车品牌持续加快全球化布局", summary: "多家汽车企业发布海外市场新规划，产品与服务体系持续完善。", cover: "assets/images/news-main.png"},
			{title: "车企联合产业伙伴推进低碳供应链建设", summary: "围绕绿色制造、循环利用和碳管理开展协同创新。", cover: "assets/images/topic-1.png"},
			{title: "新一代智能电动平台集中亮相", summary: "面向多场景出行需求，智能化技术与整车平台深度融合。", cover: "assets/images/topic-3.png"},
			{title: "汽车企业加大研发投入培育增长新动能", summary: "关键技术攻关和创新成果转化速度进一步加快。", cover: "assets/images/topic-4.png"},
		},
		"international-cooperation": {
			{title: "中国汽车工业协会代表团赴欧洲开展行业交流", summary: "围绕产业政策、技术标准和供应链合作交换意见。", cover: "assets/images/topic-2.png"},
			{title: "中外汽车产业合作交流会在北京举行", summary: "来自多个国家和地区的代表共话开放合作新机遇。", cover: "assets/images/news-main.png"},
			{title: "汽车标准国际协调工作取得新进展", summary: "行业机构持续推动标准互认和技术规则协调。", cover: "assets/images/topic-3.png"},
			{title: "全球汽车产业链合作伙伴大会成功举办", summary: "会议发布多项跨区域合作成果。", cover: "assets/images/topic-4.png"},
		},
		"industry-policy": {
			{title: "多项政策协同促进汽车消费提质升级", summary: "围绕以旧换新、充换电设施和县域市场释放消费潜力。", cover: "assets/images/news-main.png"},
			{title: "新能源汽车下乡支持政策持续优化", summary: "公共服务配套与售后保障体系进一步健全。", cover: "assets/images/topic-1.png"},
			{title: "智能网联汽车试点政策解读发布", summary: "政策进一步明确安全管理和规模化应用要求。", cover: "assets/images/topic-3.png"},
			{title: "绿色制造政策推动汽车产业低碳转型", summary: "行业加快推广节能工艺和清洁能源应用。", cover: "assets/images/topic-4.png"},
		},
		"laws-regulations": {
			{title: "汽车产品安全与召回管理制度持续完善", summary: "监管部门进一步强化产品全生命周期质量管理。", cover: "assets/images/topic-2.png"},
			{title: "道路机动车辆生产准入管理要求公开征求意见", summary: "新要求聚焦技术能力、质量保证和生产一致性。", cover: "assets/images/news-main.png"},
			{title: "汽车数据跨境合规指引发布", summary: "为企业数据处理和跨境业务提供更加清晰的规范。", cover: "assets/images/topic-3.png"},
			{title: "新能源汽车动力电池回收法规体系加快健全", summary: "生产者责任延伸和溯源管理机制进一步明确。", cover: "assets/images/topic-4.png"},
		},
	}
	for key, items := range industry {
		source.articles[key] = makeArticles(key, base.Add(-72*time.Hour), "assets/images/news-main.png", items)
	}

	statsTitles := []string{
		"新能源汽车销量分析",
		"汽车工业产销分析",
		"乘用车销量分析",
		"商用车销量分析",
		"汽车出口情况分析",
		"中国品牌乘用车市场分析",
	}
	stats := make([]model.Article, 0, len(statsTitles))
	for i, title := range statsTitles {
		stats = append(stats, model.Article{ID: fmt.Sprintf("stats-%d", i+1), Type: model.ArticleTypeData, Title: title, Summary: "2026-06", PublishTime: base.AddDate(0, 0, -i)})
	}
	source.articles["stats"] = stats

	source.articles["topics"] = makeArticles("topics", base.Add(-96*time.Hour), "assets/images/topic-site-1.jpg", []item{
		{title: "2026中国汽车论坛", summary: "聚焦产业前沿，共话高质量发展", cover: "assets/images/topic-site-1.jpg", url: "https://www.chinaautoforum.cn/"},
		{title: "汽车国报", summary: "权威行业资讯与专业观察", cover: "assets/images/topic-site-2.jpg", url: "http://www.cnautonews.com/"},
		{title: "中国汽车供应链大会", summary: "展示创新成果，促进产业协同", cover: "assets/images/topic-site-3.jpg", url: "http://chinaautoscc.cn/"},
		{title: "行业活动专题", summary: "协会重点会议与活动入口", cover: "assets/images/topic-site-4.jpg"},
	})
	source.articles["videos"] = makeArticles("videos", base.Add(-120*time.Hour), "assets/images/video-1.png", []item{
		{title: "中国汽车工业协会2026年6月信息发布会", cover: "assets/images/video-1.png"},
		{title: "2026中国汽车论坛精彩回顾", cover: "assets/images/video-2.png"},
		{title: "汽车产业运行情况专题解读", cover: "assets/images/video-3.png"},
		{title: "中国汽车品牌向上特别节目", cover: "assets/images/video-4.png"},
	})

	source.links["friend-associations"] = makeLinks("association", []item{
		{title: "日本自动车工业会", summary: "https://www.jama.or.jp/"},
		{title: "欧洲汽车工业协会", summary: "https://www.acea.auto/"},
		{title: "德国汽车工业协会", summary: "https://www.vda.de/"},
		{title: "韩国汽车产业协会", summary: "https://www.kama.or.kr/"},
	})
	source.links["friend-related"] = makeLinks("related", []item{
		{title: "工业和信息化部", summary: "https://www.miit.gov.cn/"},
		{title: "国家发展和改革委员会", summary: "https://www.ndrc.gov.cn/"},
		{title: "国家市场监督管理总局", summary: "https://www.samr.gov.cn/"},
		{title: "商务部", summary: "https://www.mofcom.gov.cn/"},
	})
	source.links["friend-media"] = makeLinks("media", []item{
		{title: "中国汽车报", summary: "https://www.cnautonews.com/"},
		{title: "人民网汽车", summary: "http://auto.people.com.cn/"},
		{title: "新华网汽车", summary: "http://auto.news.cn/"},
	})
	return source
}

func (s *Source) FetchByColumn(_ context.Context, slot config.SlotConfig) ([]model.Article, error) {
	return limitedCopy(s.articles[slot.Key], slot.Limit), nil
}

func (s *Source) ResolveColumnID(_ context.Context, slot config.SlotConfig) (int64, error) {
	return demoColumnID(slot.Key), nil
}

func (s *Source) FetchLinksByColumn(_ context.Context, slot config.SlotConfig) ([]model.Article, error) {
	return limitedCopy(s.links[slot.Key], slot.Limit), nil
}

func (s *Source) FetchMonthlyStatistics(_ context.Context, title, throughMonth string) ([]model.Article, error) {
	if strings.TrimSpace(throughMonth) == "" {
		for _, article := range s.articles["stats"] {
			if strings.TrimSpace(article.Title) == strings.TrimSpace(title) {
				throughMonth = article.Summary
				break
			}
		}
	}
	period, err := time.Parse("2006-01", strings.TrimSpace(throughMonth))
	if err != nil {
		return nil, err
	}
	seed := 18.0 + float64(statSeed(title))*4.5
	result := make([]model.Article, 0, 12+int(period.Month()))
	for month := 1; month <= 12; month++ {
		value := seed + float64(month)*1.7 + float64((month%3)*2)
		result = append(result, monthlyArticle(title, period.Year()-1, month, value))
	}
	for month := 1; month <= int(period.Month()); month++ {
		value := seed*1.08 + float64(month)*1.9 + float64((month+1)%3)*2.2
		result = append(result, monthlyArticle(title, period.Year(), month, value))
	}
	return result, nil
}

func (s *Source) FetchStatisticsTitles(_ context.Context, _ string) ([]string, error) {
	titles := make([]string, 0, len(s.articles["stats"]))
	seen := make(map[string]struct{}, len(s.articles["stats"]))
	for _, article := range s.articles["stats"] {
		title := strings.TrimSpace(article.Title)
		if title == "" {
			continue
		}
		if _, exists := seen[title]; exists {
			continue
		}
		seen[title] = struct{}{}
		titles = append(titles, title)
	}
	return titles, nil
}

func makeArticles(key string, base time.Time, fallbackCover string, items []item) []model.Article {
	result := make([]model.Article, 0, len(items))
	for i, value := range items {
		cover := value.cover
		if cover == "" {
			cover = fallbackCover
		}
		url := value.url
		if url == "" {
			url = fmt.Sprintf("article.html?id=demo-%s-%d&from=home", key, i+1)
		}
		result = append(result, model.Article{
			ID:          fmt.Sprintf("demo-%s-%d", key, i+1),
			ColumnID:    demoColumnID(key),
			Type:        model.ArticleTypeContent,
			Title:       value.title,
			Summary:     value.summary,
			Cover:       cover,
			Source:      "中国汽车工业协会",
			URL:         url,
			PublishTime: base.AddDate(0, 0, -i),
			IsBold:      i == 0 && (key == "headline" || key == "carousel"),
		})
	}
	return result
}

func demoColumnID(key string) int64 {
	var value int64 = 1000
	for _, char := range key {
		value = value*31 + int64(char)
	}
	if value < 0 {
		value = -value
	}
	return value%900000 + 1000
}

func makeLinks(key string, items []item) []model.Article {
	result := make([]model.Article, 0, len(items))
	for i, value := range items {
		result = append(result, model.Article{
			ID:    fmt.Sprintf("demo-link-%s-%d", key, i+1),
			Type:  model.ArticleTypeContent,
			Title: value.title,
			URL:   value.summary,
		})
	}
	return result
}

func monthlyArticle(title string, year, month int, value float64) model.Article {
	return model.Article{
		ID:      fmt.Sprintf("demo-stat-%d-%02d-%d", year, month, statSeed(title)),
		Type:    model.ArticleTypeData,
		Title:   title,
		Summary: fmt.Sprintf("%d-%02d", year, month),
		Content: strconv.FormatFloat(value, 'f', 1, 64),
	}
}

func statSeed(title string) int {
	titles := []string{"新能源汽车", "汽车工业", "乘用车", "商用车", "出口", "中国品牌"}
	for i, prefix := range titles {
		if strings.Contains(title, prefix) {
			return i + 1
		}
	}
	return 1
}

func limitedCopy(items []model.Article, limit int) []model.Article {
	if limit < len(items) {
		items = items[:limit]
	}
	return append([]model.Article(nil), items...)
}
