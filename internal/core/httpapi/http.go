package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"portal-static/internal/contracts"
)

type StaticGenerator = contracts.Generator

type API struct {
	operations Operations
	token      string
	timeout    time.Duration
	logger     *slog.Logger
	jobs       *jobManager
}

func New(service context.Context, g StaticGenerator, token string, timeout, idle, max time.Duration, logger *slog.Logger) http.Handler {
	return NewWithOperations(service, OperationsForGenerator(g, normalizeMIICPageName, "页面名必须是：资讯动态、核心业务、服务平台或关于我们"), token, timeout, idle, max, logger)
}

func NewWithOperations(service context.Context, operations Operations, token string, timeout, idle, max time.Duration, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	operations.Messages = operations.Messages.withDefaults()
	api := &API{operations: operations, token: token, timeout: timeout, logger: logger, jobs: newJobManager(service, idle, max)}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", api.health)
	mux.HandleFunc("/api/static/site", api.batch("site", true, operations.GenerateSite))
	mux.HandleFunc("/api/static/pages", api.batch("pages", true, operations.GeneratePages))
	mux.HandleFunc("/api/static/lists", api.batch("lists", false, operations.GenerateAllLists))
	mux.HandleFunc("/api/static/articles", api.batch("articles", false, operations.GenerateAllArticles))
	mux.HandleFunc("/api/static/topics", api.batch("topics", false, operations.GenerateTopics))
	mux.HandleFunc("/api/static/page", api.page)
	mux.HandleFunc("/api/static/list", api.list)
	mux.HandleFunc("/api/static/article", api.article)
	mux.HandleFunc("/api/static/topic", api.topic)
	mux.HandleFunc("/api/static/jobs/", api.job)
	return mux
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) batch(kind string, grayAllowed bool, run Operation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.authorize(w, r, http.MethodPost) {
			return
		}
		if run == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "generation unavailable"})
			return
		}
		output, ok := a.requestOutputPath(w, r)
		if !ok {
			return
		}
		gray, ok := a.parseGray(w, r, grayAllowed)
		if !ok {
			return
		}
		job, err := a.jobs.start(kind, func(ctx context.Context) (any, error) {
			ctx = contracts.WithOptions(ctx, output, gray)
			return run(ctx)
		})
		if err != nil {
			var busy *busyJobError
			if errors.As(err, &busy) {
				writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error(), "active_job": busy.job})
				return
			}
			a.logger.Error("创建静态化任务失败", "kind", kind, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "create generation job failed"})
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "job": job})
	}
}

func (a *API) page(w http.ResponseWriter, r *http.Request) {
	if !a.authorize(w, r, http.MethodPost) {
		return
	}
	if a.rejectWhileBatchActive(w) {
		return
	}
	requestedName := strings.TrimSpace(r.URL.Query().Get("name"))
	name, valid := requestedName, requestedName != ""
	if a.operations.NormalizePageName != nil {
		name, valid = a.operations.NormalizePageName(requestedName)
	}
	if !valid || a.operations.GeneratePage == nil {
		message := a.operations.PageNameError
		if message == "" {
			message = "unsupported page"
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": message})
		return
	}
	gray, ok := a.parseGray(w, r, true)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), a.timeout)
	defer cancel()
	output, ok := a.requestOutputPath(w, r)
	if !ok {
		return
	}
	ctx = contracts.WithOptions(ctx, output, gray)
	result, err := a.operations.GeneratePage(ctx, name)
	a.writeResult(w, result, err)
}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	if !a.authorize(w, r, http.MethodPost) {
		return
	}
	if a.rejectWhileBatchActive(w) {
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("column_name"))
	rawID := strings.TrimSpace(r.URL.Query().Get("column_id"))
	if name == "" && rawID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": a.operations.Messages.ListRequired})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), a.timeout)
	defer cancel()
	output, ok := a.requestOutputPath(w, r)
	if !ok {
		return
	}
	ctx = contracts.WithOptions(ctx, output, false)
	var result any
	var err error
	if name != "" {
		if a.operations.GenerateListByName == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": a.operations.Messages.ContentUnavailable})
			return
		}
		result, err = a.operations.GenerateListByName(ctx, name)
	} else {
		id, parseErr := strconv.ParseInt(rawID, 10, 64)
		if parseErr != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": a.operations.Messages.InvalidColumnID})
			return
		}
		if a.operations.GenerateList == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": a.operations.Messages.ContentUnavailable})
			return
		}
		result, err = a.operations.GenerateList(ctx, id)
	}
	a.writeResult(w, result, err)
}

func (a *API) article(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodPost+", "+http.MethodDelete)
		return
	}
	if !a.authorize(w, r, r.Method) {
		return
	}
	if a.rejectWhileBatchActive(w) {
		return
	}
	raw := strings.TrimSpace(r.URL.Query().Get("id"))
	if raw == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": a.operations.Messages.ArticleIDRequired})
		return
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": a.operations.Messages.InvalidArticleID})
		return
	}
	refreshRelated, ok := a.requestArticleRefresh(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), a.timeout)
	defer cancel()
	output, ok := a.requestOutputPath(w, r)
	if !ok {
		return
	}
	ctx = contracts.WithOptions(ctx, output, false)
	if r.Method == http.MethodDelete {
		var result any
		if refreshRelated {
			result, err = a.operations.DeleteArticleRelated(ctx, id)
		} else {
			result, err = a.operations.DeleteArticle(ctx, id)
		}
		a.writeResult(w, result, err)
		return
	}
	var result any
	if refreshRelated {
		result, err = a.operations.GenerateArticleRelated(ctx, id)
	} else {
		result, err = a.operations.GenerateArticle(ctx, id)
	}
	a.writeResult(w, result, err)
}

func (a *API) topic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodPost+", "+http.MethodDelete)
		return
	}
	if !a.authorize(w, r, r.Method) {
		return
	}
	if a.rejectWhileBatchActive(w) {
		return
	}
	raw := strings.TrimSpace(r.URL.Query().Get("id"))
	if raw == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": a.operations.Messages.TopicIDRequired})
		return
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": a.operations.Messages.InvalidTopicID})
		return
	}
	run := a.operations.GenerateTopic
	if r.Method == http.MethodDelete {
		run = a.operations.DeleteTopic
	}
	if run == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": a.operations.Messages.ContentUnavailable})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), a.timeout)
	defer cancel()
	output, ok := a.requestOutputPath(w, r)
	if !ok {
		return
	}
	ctx = contracts.WithOptions(ctx, output, false)
	result, err := run(ctx, id)
	a.writeResult(w, result, err)
}

func (a *API) requestArticleRefresh(w http.ResponseWriter, r *http.Request) (bool, bool) {
	switch strings.TrimSpace(r.URL.Query().Get("refresh")) {
	case "none":
		return false, true
	case "", "related":
		return true, true
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": a.operations.Messages.InvalidRefresh})
		return false, false
	}
}

func (a *API) job(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodGet+", "+http.MethodDelete)
		return
	}
	if !a.authorize(w, r, r.Method) {
		return
	}
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/static/jobs/"))
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "job not found"})
		return
	}
	var job Job
	var ok bool
	if r.Method == http.MethodDelete {
		var signaled bool
		job, ok, signaled = a.jobs.cancel(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "job not found"})
			return
		}
		status := http.StatusOK
		if signaled {
			status = http.StatusAccepted
		}
		writeJSON(w, status, map[string]any{"ok": true, "job": job})
		return
	} else {
		job, ok = a.jobs.get(id)
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "job not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "job": job})
}

func (a *API) parseGray(w http.ResponseWriter, r *http.Request, allowed bool) (bool, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("gray"))
	if raw == "" || raw == "2" {
		return false, true
	}
	if allowed && raw == "1" {
		return true, true
	}
	writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": a.operations.Messages.InvalidGray})
	return false, false
}

func (a *API) requestOutputPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("path"))
	if raw == "" {
		return "", true
	}
	clean := filepath.Clean(raw)
	windowsAbsolute := len(raw) >= 3 && ((raw[0] >= 'A' && raw[0] <= 'Z') || (raw[0] >= 'a' && raw[0] <= 'z')) && raw[1] == ':' && (raw[2] == '\\' || raw[2] == '/')
	if windowsAbsolute {
		clean = raw
	}
	if (!filepath.IsAbs(clean) && !windowsAbsolute) || (!windowsAbsolute && filepath.Dir(clean) == clean) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": a.operations.Messages.InvalidOutputPath})
		return "", false
	}
	if a.operations.ValidateOutputPath != nil {
		if err := a.operations.ValidateOutputPath(clean); err != nil {
			if errors.Is(err, contracts.ErrInvalidOutputPath) {
				writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
				return "", false
			}
			a.logger.Error("校验输出路径失败", "path", clean, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "output path validation failed"})
			return "", false
		}
	}
	return clean, true
}

func (a *API) rejectWhileBatchActive(w http.ResponseWriter) bool {
	if job, active := a.jobs.active(); active {
		writeJSON(w, http.StatusConflict, map[string]any{
			"ok": false, "error": "batch generation is already running", "active_job": job,
		})
		return true
	}
	return false
}

func (a *API) authorize(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		methodNotAllowed(w, method)
		return false
	}
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if provided == r.Header.Get("Authorization") || subtle.ConstantTimeCompare([]byte(provided), []byte(a.token)) != 1 {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
		return false
	}
	return true
}

func (a *API) writeResult(w http.ResponseWriter, result any, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result})
		return
	}
	status, message, classified := defaultErrorClassification(err)
	if a.operations.ClassifyError != nil {
		if customStatus, customMessage, ok := a.operations.ClassifyError(err); ok {
			status, message, classified = customStatus, customMessage, true
		}
	}
	if !classified {
		status, message = http.StatusInternalServerError, "static generation failed"
	}
	a.logger.Error("静态化请求失败", "error", err)
	writeJSON(w, status, map[string]any{"ok": false, "error": message})
}

func normalizeMIICPageName(name string) (string, bool) {
	switch strings.TrimSpace(name) {
	case "资讯动态", "news":
		return "news", true
	case "核心业务", "business":
		return "business", true
	case "服务平台", "platforms":
		return "platforms", true
	case "关于我们", "about":
		return "about", true
	default:
		return "", false
	}
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method not allowed"})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
