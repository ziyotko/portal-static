package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	generator "portal-static/internal/contracts"
)

type fakeGenerator struct{}

type rejectingPathGenerator struct{ fakeGenerator }

func (rejectingPathGenerator) ValidateOutputPath(string) error {
	return errors.Join(generator.ErrInvalidOutputPath, errors.New("source overlap"))
}

type blockingGenerator struct {
	fakeGenerator
	release <-chan struct{}
}

type recordingGenerator struct {
	fakeGenerator
	plainCalls   *int
	relatedCalls *int
}

func (g recordingGenerator) GenerateArticle(context.Context, int64) (generator.ArticleResult, error) {
	*g.plainCalls++
	return generator.ArticleResult{ArticleID: 1}, nil
}

func (g recordingGenerator) GenerateArticleRelated(context.Context, int64) (generator.ArticleResult, error) {
	*g.relatedCalls++
	return generator.ArticleResult{ArticleID: 1, RefreshedColumnIDs: []int64{21, 22}}, nil
}

type publishedDeleteGenerator struct{ fakeGenerator }

func (publishedDeleteGenerator) DeleteArticleRelated(context.Context, int64) (generator.DeleteArticleResult, error) {
	return generator.DeleteArticleResult{}, generator.ErrArticleStillPublished
}

func (g blockingGenerator) GenerateSite(context.Context) (generator.GenerationResult, error) {
	<-g.release
	return generator.GenerationResult{GeneratedFiles: 4}, nil
}

func (fakeGenerator) GenerateSite(context.Context) (generator.GenerationResult, error) {
	return generator.GenerationResult{DurationSeconds: 1.5, GeneratedFiles: 10, GeneratedDetails: 6, GeneratedLists: 0, GeneratedPages: 4}, nil
}
func (fakeGenerator) GeneratePages(context.Context) (generator.GenerationResult, error) {
	return generator.GenerationResult{GeneratedPages: 4}, nil
}
func (fakeGenerator) GeneratePage(context.Context, string) (generator.GenerationResult, error) {
	return generator.GenerationResult{DurationSeconds: 0.25, GeneratedFiles: 1, GeneratedPages: 1}, nil
}
func (fakeGenerator) GenerateAllLists(context.Context) (generator.GenerationResult, error) {
	return generator.GenerationResult{GeneratedColumns: 4}, nil
}
func (fakeGenerator) GenerateAllArticles(context.Context) (generator.GenerationResult, error) {
	return generator.GenerationResult{GeneratedArticles: 4}, nil
}
func (fakeGenerator) GenerateListByName(context.Context, string) (generator.ListResult, error) {
	return generator.ListResult{DurationSeconds: 0.2, GeneratedFiles: 2, GeneratedLists: 2, ColumnID: 1}, nil
}
func (fakeGenerator) GenerateList(context.Context, int64) (generator.ListResult, error) {
	return generator.ListResult{ColumnID: 1}, nil
}
func (fakeGenerator) GenerateArticle(context.Context, int64) (generator.ArticleResult, error) {
	return generator.ArticleResult{DurationSeconds: 0.1, GeneratedFiles: 1, GeneratedDetails: 1, ArticleID: 1}, nil
}
func (fakeGenerator) DeleteArticle(context.Context, int64) (generator.DeleteArticleResult, error) {
	return generator.DeleteArticleResult{ArticleID: 1}, nil
}
func (fakeGenerator) GenerateArticleRelated(context.Context, int64) (generator.ArticleResult, error) {
	return generator.ArticleResult{DurationSeconds: 0.2, GeneratedFiles: 4, GeneratedDetails: 1, GeneratedLists: 1, ArticleID: 1, RefreshedColumnIDs: []int64{21}, RefreshedPages: []string{"news"}}, nil
}
func (fakeGenerator) DeleteArticleRelated(context.Context, int64) (generator.DeleteArticleResult, error) {
	return generator.DeleteArticleResult{DurationSeconds: 0.2, GeneratedFiles: 3, GeneratedLists: 1, ArticleID: 1, RefreshedColumnIDs: []int64{21}, RefreshedPages: []string{"news"}}, nil
}

func newTestAPI() http.Handler {
	return New(context.Background(), fakeGenerator{}, "secret", time.Second, time.Minute, time.Hour, nil)
}
func authorized(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set("Authorization", "Bearer secret")
	return r
}

func TestHealthDoesNotRequireAuth(t *testing.T) {
	w := httptest.NewRecorder()
	newTestAPI().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
}
func TestStaticEndpointRequiresBearerToken(t *testing.T) {
	w := httptest.NewRecorder()
	newTestAPI().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/static/page?name=news", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
}
func TestPageRejectsUnknownName(t *testing.T) {
	w := httptest.NewRecorder()
	newTestAPI().ServeHTTP(w, authorized(http.MethodPost, "/api/static/page?name=unknown"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
}

func TestSynchronousResponsesExposeUnifiedGenerationFields(t *testing.T) {
	for _, path := range []string{
		"/api/static/page?name=news",
		"/api/static/list?column_name=latest",
		"/api/static/article?id=1",
	} {
		w := httptest.NewRecorder()
		newTestAPI().ServeHTTP(w, authorized(http.MethodPost, path))
		if w.Code != http.StatusOK {
			t.Fatalf("%s status %d: %s", path, w.Code, w.Body.String())
		}
		for _, field := range []string{"duration_seconds", "generated_files", "generated_details", "generated_lists"} {
			if !strings.Contains(w.Body.String(), `"`+field+`"`) {
				t.Fatalf("%s missing %s: %s", path, field, w.Body.String())
			}
		}
	}
}

func TestArticleDefaultsToRelatedRefreshWithoutColumns(t *testing.T) {
	plain, related := 0, 0
	api := New(context.Background(), recordingGenerator{plainCalls: &plain, relatedCalls: &related}, "secret", time.Second, time.Minute, time.Hour, nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, authorized(http.MethodPost, "/api/static/article?id=1"))
	if w.Code != http.StatusOK || plain != 0 || related != 1 {
		t.Fatalf("status=%d plain=%d related=%d body=%s", w.Code, plain, related, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"refreshed_column_ids":[21,22]`) {
		t.Fatalf("missing resolved columns: %s", w.Body.String())
	}
}

func TestArticleRefreshNoneKeepsDetailOnlyMaintenanceMode(t *testing.T) {
	plain, related := 0, 0
	api := New(context.Background(), recordingGenerator{plainCalls: &plain, relatedCalls: &related}, "secret", time.Second, time.Minute, time.Hour, nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, authorized(http.MethodPost, "/api/static/article?id=1&refresh=none"))
	if w.Code != http.StatusOK || plain != 1 || related != 0 {
		t.Fatalf("status=%d plain=%d related=%d body=%s", w.Code, plain, related, w.Body.String())
	}
}

func TestRelatedDeleteResolvesPreviousColumns(t *testing.T) {
	w := httptest.NewRecorder()
	newTestAPI().ServeHTTP(w, authorized(http.MethodDelete, "/api/static/article?id=1"))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"refreshed_column_ids":[21]`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestRelatedDeleteRejectsPublishedArticle(t *testing.T) {
	api := New(context.Background(), publishedDeleteGenerator{}, "secret", time.Second, time.Minute, time.Hour, nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, authorized(http.MethodDelete, "/api/static/article?id=1"))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "先在数据库下架") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestArticleRejectsUnknownRefreshMode(t *testing.T) {
	for _, path := range []string{
		"/api/static/article?id=1&refresh=all",
	} {
		w := httptest.NewRecorder()
		newTestAPI().ServeHTTP(w, authorized(http.MethodPost, path))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
}

func TestOutputPathValidatorRejectsUnsafePathBeforeJobStarts(t *testing.T) {
	api := New(context.Background(), rejectingPathGenerator{}, "secret", time.Second, time.Minute, time.Hour, nil)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, authorized(http.MethodPost, "/api/static/site?path=D%3A%5Cunsafe"))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid output path") {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}

func TestBatchConflictReturnsActiveJobAndBlocksSynchronousWrites(t *testing.T) {
	release := make(chan struct{})
	api := New(context.Background(), blockingGenerator{release: release}, "secret", time.Second, time.Minute, time.Hour, nil)
	started := httptest.NewRecorder()
	api.ServeHTTP(started, authorized(http.MethodPost, "/api/static/site"))
	if started.Code != http.StatusAccepted {
		t.Fatalf("start status %d: %s", started.Code, started.Body.String())
	}
	for _, path := range []string{"/api/static/pages", "/api/static/page?name=news"} {
		conflict := httptest.NewRecorder()
		api.ServeHTTP(conflict, authorized(http.MethodPost, path))
		if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), `"active_job"`) {
			t.Fatalf("%s conflict status %d: %s", path, conflict.Code, conflict.Body.String())
		}
	}
	close(release)
}
func TestBatchReturnsJobAndCanBeQueried(t *testing.T) {
	api := newTestAPI()
	w := httptest.NewRecorder()
	api.ServeHTTP(w, authorized(http.MethodPost, "/api/static/site"))
	if w.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Job Job `json:"job"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	query := httptest.NewRecorder()
	api.ServeHTTP(query, authorized(http.MethodGet, response.Job.StatusURL))
	if query.Code != http.StatusOK {
		t.Fatalf("query status %d: %s", query.Code, query.Body.String())
	}
}

func TestCancelActiveBatchReturnsAccepted(t *testing.T) {
	release := make(chan struct{})
	api := New(context.Background(), blockingGenerator{release: release}, "secret", time.Second, time.Minute, time.Hour, nil)
	started := httptest.NewRecorder()
	api.ServeHTTP(started, authorized(http.MethodPost, "/api/static/site"))
	var response struct {
		Job Job `json:"job"`
	}
	if err := json.Unmarshal(started.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	cancelled := httptest.NewRecorder()
	api.ServeHTTP(cancelled, authorized(http.MethodDelete, response.Job.CancelURL))
	if cancelled.Code != http.StatusAccepted || !strings.Contains(cancelled.Body.String(), "正在取消") {
		t.Fatalf("cancel status %d: %s", cancelled.Code, cancelled.Body.String())
	}
	close(release)
}
func TestBatchRejectsRelativeOutputPath(t *testing.T) {
	w := httptest.NewRecorder()
	newTestAPI().ServeHTTP(w, authorized(http.MethodPost, "/api/static/site?path=relative"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
}
