package contracttest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portal-static/internal/contracts"
	"portal-static/internal/core/httpapi"
)

type PageAlias struct {
	Input      string
	Normalized string
}

// Run executes the public static-generation contract against an adapter's
// operations. New adapters should call this helper from their tests.
func Run(t *testing.T, operations httpapi.Operations, aliases []PageAlias) {
	t.Helper()
	if err := operations.Validate(); err != nil {
		t.Fatalf("operations contract: %v", err)
	}

	t.Run("authentication", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler(operations, time.Second).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/static/site", nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})

	t.Run("batch_jobs", func(t *testing.T) {
		batchOps := operations
		complete := func(context.Context) (any, error) { return map[string]any{"generated_files": 1}, nil }
		batchOps.GenerateSite = complete
		batchOps.GeneratePages = complete
		batchOps.GenerateAllLists = complete
		batchOps.GenerateAllArticles = complete
		batchOps.GenerateTopics = complete
		for _, endpoint := range []string{"site", "pages", "lists", "articles", "topics"} {
			response := httptest.NewRecorder()
			handler(batchOps, time.Second).ServeHTTP(response, authorized(http.MethodPost, "/api/static/"+endpoint))
			if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"status_url"`) || !strings.Contains(response.Body.String(), `"cancel_url"`) {
				t.Fatalf("%s status=%d body=%s", endpoint, response.Code, response.Body.String())
			}
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		cancelOps := operations
		cancelOps.GenerateSite = func(ctx context.Context) (any, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		api := handler(cancelOps, time.Second)
		started := httptest.NewRecorder()
		api.ServeHTTP(started, authorized(http.MethodPost, "/api/static/site"))
		var payload struct {
			Job struct {
				CancelURL string `json:"cancel_url"`
			} `json:"job"`
		}
		if err := json.Unmarshal(started.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		cancelled := httptest.NewRecorder()
		api.ServeHTTP(cancelled, authorized(http.MethodDelete, payload.Job.CancelURL))
		if cancelled.Code != http.StatusAccepted {
			t.Fatalf("status=%d body=%s", cancelled.Code, cancelled.Body.String())
		}
	})

	t.Run("page_aliases_gray_and_output", func(t *testing.T) {
		for _, alias := range aliases {
			alias := alias
			t.Run(alias.Input, func(t *testing.T) {
				called := ""
				var options contracts.Options
				pageOps := operations
				pageOps.ValidateOutputPath = func(string) error { return nil }
				pageOps.GeneratePage = func(ctx context.Context, name string) (any, error) {
					called = name
					options = contracts.OptionsFrom(ctx)
					return map[string]any{"generated_files": 1}, nil
				}
				output := filepath.Join(t.TempDir(), "preview")
				target := "/api/static/page?name=" + url.QueryEscape(alias.Input) + "&gray=1&path=" + url.QueryEscape(output)
				response := httptest.NewRecorder()
				handler(pageOps, time.Second).ServeHTTP(response, authorized(http.MethodPost, target))
				if response.Code != http.StatusOK || called != alias.Normalized || !options.Grayscale || options.OutputPath != output {
					t.Fatalf("status=%d called=%q options=%+v body=%s", response.Code, called, options, response.Body.String())
				}
			})
		}
	})

	t.Run("invalid_page_gray_and_path", func(t *testing.T) {
		for _, target := range []string{
			"/api/static/page?name=not-a-page",
			"/api/static/page?name=" + url.QueryEscape(aliases[0].Input) + "&gray=true",
			"/api/static/site?path=relative",
		} {
			response := httptest.NewRecorder()
			handler(operations, time.Second).ServeHTTP(response, authorized(http.MethodPost, target))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("%s status=%d body=%s", target, response.Code, response.Body.String())
			}
		}
	})

	t.Run("topic_generate_delete_and_validation", func(t *testing.T) {
		topicOps := operations
		topicOps.ValidateOutputPath = func(string) error { return nil }
		var generated, deleted int64
		topicOps.GenerateTopic = func(_ context.Context, id int64) (any, error) {
			generated = id
			return map[string]any{"generated_files": 1, "template_id": id}, nil
		}
		topicOps.DeleteTopic = func(_ context.Context, id int64) (any, error) {
			deleted = id
			return map[string]any{"deleted": true, "template_id": id}, nil
		}
		api := handler(topicOps, time.Second)
		for _, method := range []string{http.MethodPost, http.MethodDelete} {
			response := httptest.NewRecorder()
			api.ServeHTTP(response, authorized(method, "/api/static/topic?id=17"))
			if response.Code != http.StatusOK {
				t.Fatalf("%s status=%d body=%s", method, response.Code, response.Body.String())
			}
		}
		if generated != 17 || deleted != 17 {
			t.Fatalf("generated=%d deleted=%d", generated, deleted)
		}
		invalid := httptest.NewRecorder()
		api.ServeHTTP(invalid, authorized(http.MethodPost, "/api/static/topic?id=../17"))
		if invalid.Code != http.StatusBadRequest {
			t.Fatalf("invalid topic status=%d body=%s", invalid.Code, invalid.Body.String())
		}
	})

	t.Run("error_classification", func(t *testing.T) {
		errorOps := operations
		errorOps.GenerateList = func(context.Context, int64) (any, error) { return nil, contracts.ErrColumnNotFound }
		response := httptest.NewRecorder()
		handler(errorOps, time.Second).ServeHTTP(response, authorized(http.MethodPost, "/api/static/list?column_id=1"))
		if response.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})

	t.Run("request_timeout", func(t *testing.T) {
		timeoutOps := operations
		timeoutOps.GeneratePage = func(ctx context.Context, _ string) (any, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		response := httptest.NewRecorder()
		handler(timeoutOps, 10*time.Millisecond).ServeHTTP(response, authorized(http.MethodPost, "/api/static/page?name="+url.QueryEscape(aliases[0].Input)))
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})
}

func handler(operations httpapi.Operations, timeout time.Duration) http.Handler {
	return httpapi.NewWithOperations(context.Background(), operations, "secret", timeout, time.Minute, time.Hour, nil)
}

func authorized(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer secret")
	return request
}
