package camie

import (
	"context"
	"errors"
	"math"
	"time"

	"portal-static/internal/adapters/camie/generator"
	"portal-static/internal/contracts"
)

// Service implements portal-static's shared job contract. CAMIE publishes an
// atomic site snapshot, so scoped jobs intentionally rebuild the snapshot in
// this first merged version. That keeps deletion and related-page refreshes
// correct without maintaining a second, adapter-specific task API.
type Service struct {
	g        *generator.Generator
	pageSize int
}

func NewService(g *generator.Generator, pageSize int) *Service {
	return &Service{g: g, pageSize: pageSize}
}

func (s *Service) rebuild(ctx context.Context) (contracts.GenerationResult, error) {
	started := time.Now()
	result, err := s.g.GenerateSite(ctx)
	if err != nil {
		return contracts.GenerationResult{}, err
	}
	gray := "2"
	if contracts.OptionsFrom(ctx).Grayscale {
		gray = "1"
	}
	return contracts.GenerationResult{
		GeneratedAt: result.GeneratedAt, DurationSeconds: time.Since(started).Seconds(), GeneratedFiles: result.Files,
		GeneratedDetails: result.Articles, GeneratedLists: result.Lists, GeneratedPages: 3,
		GeneratedColumns: result.Lists, GeneratedArticles: result.Articles, Output: result.Output, Gray: gray,
	}, nil
}

func (s *Service) GenerateSite(ctx context.Context) (contracts.GenerationResult, error) {
	return s.rebuild(ctx)
}
func (s *Service) GeneratePages(ctx context.Context) (contracts.GenerationResult, error) {
	return s.rebuild(ctx)
}
func (s *Service) GenerateAllLists(ctx context.Context) (contracts.GenerationResult, error) {
	return s.rebuild(ctx)
}
func (s *Service) GenerateAllArticles(ctx context.Context) (contracts.GenerationResult, error) {
	return s.rebuild(ctx)
}

func (s *Service) GeneratePage(ctx context.Context, _ string) (contracts.GenerationResult, error) {
	return s.rebuild(ctx)
}

func (s *Service) GenerateListByName(ctx context.Context, name string) (contracts.ListResult, error) {
	column, total, err := s.g.Column(ctx, name, 0)
	if err != nil {
		return contracts.ListResult{}, err
	}
	return s.generateList(ctx, column.ID, total)
}

func (s *Service) GenerateList(ctx context.Context, id int64) (contracts.ListResult, error) {
	column, total, err := s.g.Column(ctx, "", id)
	if err != nil {
		return contracts.ListResult{}, err
	}
	return s.generateList(ctx, column.ID, total)
}

func (s *Service) generateList(ctx context.Context, id int64, total int) (contracts.ListResult, error) {
	started := time.Now()
	result, err := s.rebuild(ctx)
	if err != nil {
		return contracts.ListResult{}, err
	}
	pages := int(math.Ceil(float64(total) / float64(s.pageSize)))
	if pages < 1 {
		pages = 1
	}
	return contracts.ListResult{GeneratedAt: result.GeneratedAt, DurationSeconds: time.Since(started).Seconds(), GeneratedFiles: result.GeneratedFiles, GeneratedDetails: result.GeneratedDetails, GeneratedLists: result.GeneratedLists, ColumnID: id, TotalItems: total, TotalPages: pages, PageSize: s.pageSize, Output: result.Output}, nil
}

func (s *Service) GenerateArticle(ctx context.Context, id int64) (contracts.ArticleResult, error) {
	article, err := s.g.Article(ctx, id)
	if err != nil {
		return contracts.ArticleResult{}, err
	}
	return s.generateArticle(ctx, article.ID, article.ColumnCode)
}

func (s *Service) generateArticle(ctx context.Context, id int64, code string) (contracts.ArticleResult, error) {
	column, _, err := s.g.Column(ctx, code, 0)
	if err != nil {
		return contracts.ArticleResult{}, err
	}
	started := time.Now()
	result, err := s.rebuild(ctx)
	if err != nil {
		return contracts.ArticleResult{}, err
	}
	return contracts.ArticleResult{GeneratedAt: result.GeneratedAt, DurationSeconds: time.Since(started).Seconds(), GeneratedFiles: result.GeneratedFiles, GeneratedDetails: result.GeneratedDetails, GeneratedLists: result.GeneratedLists, ArticleID: id, ColumnID: column.ID, Output: result.Output, RefreshedColumnIDs: []int64{column.ID}, RefreshedPages: []string{"home", "search"}}, nil
}

func (s *Service) DeleteArticle(ctx context.Context, id int64) (contracts.DeleteArticleResult, error) {
	if _, err := s.g.Article(ctx, id); err == nil {
		return contracts.DeleteArticleResult{}, contracts.ErrArticleStillPublished
	} else if !errors.Is(err, contracts.ErrArticleNotPublished) {
		return contracts.DeleteArticleResult{}, err
	}
	started := time.Now()
	result, err := s.rebuild(ctx)
	if err != nil {
		return contracts.DeleteArticleResult{}, err
	}
	return contracts.DeleteArticleResult{DeletedAt: time.Now(), DurationSeconds: time.Since(started).Seconds(), GeneratedFiles: result.GeneratedFiles, GeneratedDetails: result.GeneratedDetails, GeneratedLists: result.GeneratedLists, ArticleID: id, Deleted: true, RefreshedPages: []string{"home", "search"}}, nil
}

func (s *Service) GenerateArticleRelated(ctx context.Context, id int64) (contracts.ArticleResult, error) {
	return s.GenerateArticle(ctx, id)
}
func (s *Service) DeleteArticleRelated(ctx context.Context, id int64) (contracts.DeleteArticleResult, error) {
	return s.DeleteArticle(ctx, id)
}
func (s *Service) ValidateOutputPath(path string) error { return s.g.ValidateOutputPath(path) }

var _ contracts.Generator = (*Service)(nil)
