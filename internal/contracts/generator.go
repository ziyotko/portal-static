package contracts

import (
	"context"
	"errors"
	"time"
)

var (
	ErrBusy                  = errors.New("static generation is already running")
	ErrInvalidOutputPath     = errors.New("invalid output path")
	ErrArticleStillPublished = errors.New("article is still publishable")
)

type Generator interface {
	GenerateSite(context.Context) (GenerationResult, error)
	GeneratePages(context.Context) (GenerationResult, error)
	GeneratePage(context.Context, string) (GenerationResult, error)
	GenerateAllLists(context.Context) (GenerationResult, error)
	GenerateAllArticles(context.Context) (GenerationResult, error)
	GenerateListByName(context.Context, string) (ListResult, error)
	GenerateList(context.Context, int64) (ListResult, error)
	GenerateArticle(context.Context, int64) (ArticleResult, error)
	DeleteArticle(context.Context, int64) (DeleteArticleResult, error)
	GenerateArticleRelated(context.Context, int64) (ArticleResult, error)
	DeleteArticleRelated(context.Context, int64) (DeleteArticleResult, error)
}

type GenerationResult struct {
	GeneratedAt       time.Time `json:"generated_at"`
	DurationSeconds   float64   `json:"duration_seconds"`
	GeneratedFiles    int       `json:"generated_files"`
	GeneratedDetails  int       `json:"generated_details"`
	GeneratedLists    int       `json:"generated_lists"`
	Output            string    `json:"output"`
	GeneratedPages    int       `json:"generated_pages,omitempty"`
	GeneratedColumns  int       `json:"generated_columns,omitempty"`
	GeneratedArticles int       `json:"generated_articles,omitempty"`
	TotalItems        int       `json:"total_items,omitempty"`
	TotalPages        int       `json:"total_pages,omitempty"`
	Gray              string    `json:"gray,omitempty"`
}

type ArticleResult struct {
	GeneratedAt        time.Time `json:"generated_at"`
	DurationSeconds    float64   `json:"duration_seconds"`
	GeneratedFiles     int       `json:"generated_files"`
	GeneratedDetails   int       `json:"generated_details"`
	GeneratedLists     int       `json:"generated_lists"`
	ArticleID          int64     `json:"article_id"`
	ColumnID           int64     `json:"column_id"`
	Output             string    `json:"output"`
	RefreshedColumnIDs []int64   `json:"refreshed_column_ids,omitempty"`
	RefreshedPages     []string  `json:"refreshed_pages,omitempty"`
}

type DeleteArticleResult struct {
	DeletedAt          time.Time `json:"deleted_at"`
	DurationSeconds    float64   `json:"duration_seconds"`
	GeneratedFiles     int       `json:"generated_files"`
	GeneratedDetails   int       `json:"generated_details"`
	GeneratedLists     int       `json:"generated_lists"`
	ArticleID          int64     `json:"article_id"`
	Deleted            bool      `json:"deleted"`
	DeletedPaths       []string  `json:"deleted_paths"`
	RefreshedColumnIDs []int64   `json:"refreshed_column_ids,omitempty"`
	RefreshedPages     []string  `json:"refreshed_pages,omitempty"`
}

type ListResult struct {
	GeneratedAt      time.Time `json:"generated_at"`
	DurationSeconds  float64   `json:"duration_seconds"`
	GeneratedFiles   int       `json:"generated_files"`
	GeneratedDetails int       `json:"generated_details"`
	GeneratedLists   int       `json:"generated_lists"`
	ColumnID         int64     `json:"column_id"`
	TotalItems       int       `json:"total_items"`
	TotalPages       int       `json:"total_pages"`
	PageSize         int       `json:"page_size"`
	Output           string    `json:"output"`
}

type Progress struct {
	Stage            string `json:"stage"`
	Processed        int    `json:"processed"`
	Total            int    `json:"total"`
	CurrentColumnID  int64  `json:"current_column_id,omitempty"`
	CurrentArticleID int64  `json:"current_article_id,omitempty"`
	GeneratedFiles   int    `json:"generated_files"`
}

type optionsKey struct{}
type progressKey struct{}

type Options struct {
	OutputPath string
	Grayscale  bool
}

func WithOptions(ctx context.Context, outputPath string, grayscale bool) context.Context {
	return context.WithValue(ctx, optionsKey{}, Options{OutputPath: outputPath, Grayscale: grayscale})
}

func OptionsFrom(ctx context.Context) Options {
	value, _ := ctx.Value(optionsKey{}).(Options)
	return value
}

func WithProgress(ctx context.Context, callback func(Progress)) context.Context {
	return context.WithValue(ctx, progressKey{}, callback)
}

func ReportProgress(ctx context.Context, progress Progress) {
	if callback, ok := ctx.Value(progressKey{}).(func(Progress)); ok && callback != nil {
		callback(progress)
	}
}
