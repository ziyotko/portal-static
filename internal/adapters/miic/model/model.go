package model

import "time"

const (
	ArticleTypeContent = 1
	ArticleTypeVideo   = 2
)

func IsSupportedArticleType(articleType int) bool {
	return articleType == ArticleTypeContent || articleType == ArticleTypeVideo
}

func HasStaticDetail(articleType int) bool {
	return IsSupportedArticleType(articleType)
}

type Column struct {
	ID           int64
	Name         string
	Code         string
	TemplateID   int64
	TemplateName string
	ParentID     int64
	Description  string
	Sort         int
}

type Attachment struct {
	ID        int64
	ArticleID int64
	Name      string
	URL       string
	Size      int64
}

type Article struct {
	ID           int64
	ColumnID     int64
	Type         int
	Title        string
	Summary      string
	Content      string
	Cover        string
	Author       string
	Source       string
	IsBold       bool
	IsTop        bool
	DefaultColor string
	URL          string
	PublishTime  time.Time
}
