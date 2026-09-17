package caam

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portal-static/internal/adapters/caam/generator"
	"portal-static/internal/contracts"
	"portal-static/internal/core/httpapi"
)

type contractResult struct {
	GeneratedFiles int    `json:"generated_files"`
	Name           string `json:"name,omitempty"`
}

func (r contractResult) GeneratedFileCount() int { return r.GeneratedFiles }

type contractRecorder struct {
	pageName     string
	listID       int64
	listName     string
	articleID    int64
	articleMode  string
	options      contracts.Options
	operationErr error
}

func (r *contractRecorder) operations() httpapi.Operations {
	record := func(ctx context.Context) (any, error) {
		r.options = contracts.OptionsFrom(ctx)
		return contractResult{GeneratedFiles: 3}, r.operationErr
	}
	return withHTTPContract(httpapi.Operations{
		GenerateSite:        record,
		GeneratePages:       record,
		GenerateAllLists:    record,
		GenerateAllArticles: record,
		GeneratePage: func(ctx context.Context, name string) (any, error) {
			r.pageName = name
			r.options = contracts.OptionsFrom(ctx)
			return contractResult{GeneratedFiles: 1, Name: name}, r.operationErr
		},
		GenerateList: func(ctx context.Context, id int64) (any, error) {
			r.listID = id
			r.options = contracts.OptionsFrom(ctx)
			return contractResult{GeneratedFiles: 1}, r.operationErr
		},
		GenerateListByName: func(ctx context.Context, name string) (any, error) {
			r.listName = name
			r.options = contracts.OptionsFrom(ctx)
			return contractResult{GeneratedFiles: 1}, r.operationErr
		},
		GenerateArticle: func(ctx context.Context, id int64) (any, error) {
			r.articleID, r.articleMode = id, "plain"
			r.options = contracts.OptionsFrom(ctx)
			return contractResult{GeneratedFiles: 1}, r.operationErr
		},
		DeleteArticle: func(ctx context.Context, id int64) (any, error) {
			r.articleID, r.articleMode = id, "delete_plain"
			r.options = contracts.OptionsFrom(ctx)
			return contractResult{GeneratedFiles: 1}, r.operationErr
		},
		GenerateArticleRelated: func(ctx context.Context, id int64) (any, error) {
			r.articleID, r.articleMode = id, "related"
			r.options = contracts.OptionsFrom(ctx)
			return contractResult{GeneratedFiles: 3}, r.operationErr
		},
		DeleteArticleRelated: func(ctx context.Context, id int64) (any, error) {
			r.articleID, r.articleMode = id, "delete_related"
			r.options = contracts.OptionsFrom(ctx)
			return contractResult{GeneratedFiles: 3}, r.operationErr
		},
	})
}

func newContractHandler(recorder *contractRecorder) http.Handler {
	return httpapi.NewWithOperations(context.Background(), recorder.operations(), "secret", time.Second, time.Minute, time.Hour, nil)
}

func contractRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer secret")
	return request
}

func TestCAAMHTTPContractKeepsOnlyRegisteredPublicRoutes(t *testing.T) {
	handler := newContractHandler(&contractRecorder{})
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/api/static/page?name=home", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	for _, path := range []string{"/api/static/home", "/api/static/home/articles", "/api/static/site/articles"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, contractRequest(http.MethodPost, path))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, response.Code)
		}
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, contractRequest(http.MethodPost, "/api/static/site"))
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"status_url"`) {
		t.Fatalf("site status = %d: %s", response.Code, response.Body.String())
	}
}

func TestCAAMHTTPContractAcceptsChineseAndEnglishPageNames(t *testing.T) {
	for input, want := range map[string]string{
		"首页": "home", "协会概况": "about", "协会工作": "work",
		"统计数据": "stats", "会员专区": "members", "党建专区": "party",
		"home": "home", "about": "about", "work": "work",
		"stats": "stats", "members": "members", "party": "party",
	} {
		recorder := &contractRecorder{}
		output := filepath.Join(t.TempDir(), "preview")
		target := "/api/static/page?name=" + url.QueryEscape(input) + "&gray=1&path=" + url.QueryEscape(output)
		response := httptest.NewRecorder()
		newContractHandler(recorder).ServeHTTP(response, contractRequest(http.MethodPost, target))
		if response.Code != http.StatusOK || recorder.pageName != want {
			t.Fatalf("%q status=%d name=%q body=%s", input, response.Code, recorder.pageName, response.Body.String())
		}
		if recorder.options.OutputPath != output || !recorder.options.Grayscale {
			t.Fatalf("%q options = %#v", input, recorder.options)
		}
	}

	response := httptest.NewRecorder()
	newContractHandler(&contractRecorder{}).ServeHTTP(response, contractRequest(http.MethodPost, "/api/static/page?name=contact"))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "页面名必须是") {
		t.Fatalf("invalid page status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCAAMHTTPContractPreservesListAndArticleParameters(t *testing.T) {
	recorder := &contractRecorder{}
	handler := newContractHandler(recorder)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, contractRequest(http.MethodPost, "/api/static/list?column_name="+url.QueryEscape("行业要闻")))
	if response.Code != http.StatusOK || recorder.listName != "行业要闻" {
		t.Fatalf("named list status=%d name=%q", response.Code, recorder.listName)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, contractRequest(http.MethodPost, "/api/static/list?column_id=42"))
	if response.Code != http.StatusOK || recorder.listID != 42 {
		t.Fatalf("id list status=%d id=%d", response.Code, recorder.listID)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, contractRequest(http.MethodPost, "/api/static/article?id=101"))
	if response.Code != http.StatusOK || recorder.articleID != 101 || recorder.articleMode != "related" {
		t.Fatalf("related article status=%d id=%d mode=%q", response.Code, recorder.articleID, recorder.articleMode)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, contractRequest(http.MethodPost, "/api/static/article?id=101&refresh=none"))
	if response.Code != http.StatusOK || recorder.articleMode != "plain" {
		t.Fatalf("plain article status=%d mode=%q", response.Code, recorder.articleMode)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, contractRequest(http.MethodDelete, "/api/static/article?id=101"))
	if response.Code != http.StatusOK || recorder.articleMode != "delete_related" {
		t.Fatalf("related delete status=%d mode=%q", response.Code, recorder.articleMode)
	}
}

func TestCAAMHTTPContractPreservesValidationMessages(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/static/list", "column_name（栏目名称）不能为空"},
		{"/api/static/list?column_id=bad", "column_id 必须是正整数"},
		{"/api/static/article", "id is required"},
		{"/api/static/article?id=bad", "id must be a positive integer"},
		{"/api/static/article?id=1&refresh=all", "refresh must be related or none when provided"},
		{"/api/static/page?name=home&gray=true", "gray must be the string 1 or 2"},
		{"/api/static/list?column_id=1&path=relative", "path must be an absolute non-root directory"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		newContractHandler(&contractRecorder{}).ServeHTTP(response, contractRequest(http.MethodPost, test.path))
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), test.want) {
			t.Errorf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}

func TestCAAMHTTPContractClassifiesContentErrors(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{
		{generator.ErrBusy, http.StatusConflict},
		{generator.ErrColumnNotUnique, http.StatusConflict},
		{generator.ErrColumnNotFound, http.StatusNotFound},
		{generator.ErrArticleNotPublished, http.StatusNotFound},
		{generator.ErrArticleStillPublished, http.StatusConflict},
		{errors.New("unexpected"), http.StatusInternalServerError},
	} {
		recorder := &contractRecorder{operationErr: test.err}
		response := httptest.NewRecorder()
		newContractHandler(recorder).ServeHTTP(response, contractRequest(http.MethodPost, "/api/static/list?column_id=1"))
		if response.Code != test.status {
			t.Errorf("error %v status=%d body=%s", test.err, response.Code, response.Body.String())
		}
	}
}
