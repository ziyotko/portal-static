package model

import "time"

const (
	ArticleTypeContent = 1
	ArticleTypeVideo   = 2
	ArticleTypeData    = 3
)

func IsSupportedArticleType(articleType int) bool {
	return articleType == ArticleTypeContent || articleType == ArticleTypeVideo || articleType == ArticleTypeData
}

func HasStaticDetail(articleType int) bool {
	return articleType == ArticleTypeContent || articleType == ArticleTypeVideo
}

type Article struct {
	ID           string
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

type ArticleColumnMapping struct {
	ID        int64
	ColumnID  int64
	ArticleID int64
}

type Column struct {
	ID   int64
	Name string
}

type Series struct {
	Name   string    `json:"name"`
	Value  float64   `json:"value"`
	Months []float64 `json:"months"`
}

type StatsPayload struct {
	Label  string   `json:"label"`
	Unit   string   `json:"unit"`
	Series []Series `json:"series"`
}
