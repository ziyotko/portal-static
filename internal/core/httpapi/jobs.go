package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"portal-static/internal/contracts"
)

const (
	jobQueued      = "queued"
	jobRunning     = "running"
	jobSucceeded   = "succeeded"
	jobFailed      = "failed"
	jobInterrupted = "interrupted"
	jobRetention   = 24 * time.Hour
	maxJobs        = 100
)

var (
	errJobCanceled    = errors.New("batch job canceled by request")
	errJobIdleTimeout = errors.New("batch job made no progress")
	errJobMaxDuration = errors.New("batch job exceeded maximum duration")
)

type Job struct {
	ID         string             `json:"id"`
	Kind       string             `json:"kind"`
	Status     string             `json:"status"`
	StatusURL  string             `json:"status_url"`
	CancelURL  string             `json:"cancel_url"`
	Progress   contracts.Progress `json:"progress"`
	CreatedAt  time.Time          `json:"created_at"`
	StartedAt  *time.Time         `json:"started_at,omitempty"`
	UpdatedAt  time.Time          `json:"updated_at"`
	FinishedAt *time.Time         `json:"finished_at,omitempty"`
	Result     any                `json:"result,omitempty"`
	Error      string             `json:"error,omitempty"`
}

type busyJobError struct{ job Job }

func (e *busyJobError) Error() string {
	return fmt.Sprintf("generation job %s is already %s", e.job.ID, e.job.Status)
}

type jobManager struct {
	ctx      context.Context
	idle     time.Duration
	max      time.Duration
	mu       sync.Mutex
	jobs     map[string]*Job
	controls map[string]*jobControl
	order    []string
	activeID string
}

type jobControl struct {
	cancel   context.CancelCauseFunc
	progress chan struct{}
}

type jobOutcome struct {
	result any
	err    error
}

func newJobManager(ctx context.Context, idle, max time.Duration) *jobManager {
	if ctx == nil {
		ctx = context.Background()
	}
	if idle <= 0 {
		idle = 15 * time.Minute
	}
	if max <= 0 {
		max = 6 * time.Hour
	}
	return &jobManager{
		ctx: ctx, idle: idle, max: max,
		jobs: make(map[string]*Job), controls: make(map[string]*jobControl),
	}
}

func (m *jobManager) start(kind string, run func(context.Context) (any, error)) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(time.Now())
	if m.activeID != "" {
		if active, exists := m.jobs[m.activeID]; exists && (active.Status == jobQueued || active.Status == jobRunning) {
			return Job{}, &busyJobError{job: *active}
		}
		m.activeID = ""
	}
	id, err := newJobID()
	if err != nil {
		return Job{}, err
	}
	now := time.Now()
	job := &Job{
		ID: id, Kind: kind, Status: jobQueued,
		StatusURL: "/api/static/jobs/" + id, CancelURL: "/api/static/jobs/" + id,
		Progress: contracts.Progress{Stage: "等待执行"}, CreatedAt: now, UpdatedAt: now,
	}
	jobCtx, cancel := context.WithCancelCause(m.ctx)
	control := &jobControl{cancel: cancel, progress: make(chan struct{}, 1)}
	m.jobs[id] = job
	m.controls[id] = control
	m.order = append(m.order, id)
	m.activeID = id
	go m.run(id, jobCtx, control, run)
	return *job, nil
}

func (m *jobManager) run(id string, jobCtx context.Context, control *jobControl, run func(context.Context) (any, error)) {
	now := time.Now()
	m.mu.Lock()
	job, exists := m.jobs[id]
	if !exists {
		m.mu.Unlock()
		return
	}
	job.Status = jobRunning
	job.StartedAt = &now
	job.UpdatedAt = now
	job.Progress.Stage = "开始执行"
	m.mu.Unlock()

	ctx := contracts.WithProgress(jobCtx, func(progress contracts.Progress) {
		m.updateProgress(id, progress)
	})
	outcomes := make(chan jobOutcome, 1)
	go func() {
		outcome := jobOutcome{}
		defer func() {
			if recovered := recover(); recovered != nil {
				outcome.err = fmt.Errorf("batch job panic: %v", recovered)
			}
			outcomes <- outcome
		}()
		outcome.result, outcome.err = run(ctx)
	}()

	idleTimer := time.NewTimer(m.idle)
	maxTimer := time.NewTimer(m.max)
	defer idleTimer.Stop()
	defer maxTimer.Stop()
	for {
		select {
		case outcome := <-outcomes:
			m.finish(id, jobCtx, outcome)
			return
		case <-control.progress:
			resetTimer(idleTimer, m.idle)
		case <-idleTimer.C:
			control.cancel(fmt.Errorf("%w for %s", errJobIdleTimeout, m.idle))
		case <-maxTimer.C:
			control.cancel(fmt.Errorf("%w %s", errJobMaxDuration, m.max))
		}
	}
}

func (m *jobManager) finish(id string, jobCtx context.Context, outcome jobOutcome) {
	finished := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	job, exists := m.jobs[id]
	if !exists {
		return
	}
	job.UpdatedAt = finished
	job.FinishedAt = &finished
	if cause := context.Cause(jobCtx); cause != nil {
		job.Status = jobInterrupted
		job.Progress.Stage = "执行中断"
		job.Error = cause.Error()
	} else if outcome.err == nil {
		job.Status = jobSucceeded
		job.Progress.Stage = "生成完成"
		job.Result = outcome.result
		if generatedFiles, ok := completedGeneratedFiles(outcome.result); ok {
			job.Progress.GeneratedFiles = generatedFiles
		}
	} else if errors.Is(outcome.err, context.Canceled) || errors.Is(outcome.err, context.DeadlineExceeded) {
		job.Status = jobInterrupted
		job.Progress.Stage = "执行中断"
		job.Error = outcome.err.Error()
	} else {
		job.Status = jobFailed
		job.Progress.Stage = "生成失败"
		job.Error = outcome.err.Error()
	}
	delete(m.controls, id)
	if m.activeID == id {
		m.activeID = ""
	}
	m.cleanupLocked(finished)
}

func completedGeneratedFiles(result any) (int, bool) {
	if value, ok := result.(interface{ GeneratedFileCount() int }); ok {
		return value.GeneratedFileCount(), true
	}
	return 0, false
}

func (m *jobManager) updateProgress(id string, progress contracts.Progress) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job, exists := m.jobs[id]; exists && job.Status == jobRunning {
		job.Progress = progress
		job.UpdatedAt = time.Now()
		if control := m.controls[id]; control != nil {
			select {
			case control.progress <- struct{}{}:
			default:
			}
		}
	}
}

func (m *jobManager) cancel(id string) (Job, bool, bool) {
	m.mu.Lock()
	job, exists := m.jobs[id]
	if !exists {
		m.mu.Unlock()
		return Job{}, false, false
	}
	control := m.controls[id]
	active := job.Status == jobQueued || job.Status == jobRunning
	if active {
		job.Progress.Stage = "正在取消"
		job.UpdatedAt = time.Now()
	}
	result := *job
	m.mu.Unlock()
	if active && control != nil {
		control.cancel(errJobCanceled)
	}
	return result, true, active
}

func (m *jobManager) get(id string) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked(time.Now())
	job, exists := m.jobs[id]
	if !exists {
		return Job{}, false
	}
	return *job, true
}

func (m *jobManager) active() (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeID == "" {
		return Job{}, false
	}
	job, exists := m.jobs[m.activeID]
	if !exists || (job.Status != jobQueued && job.Status != jobRunning) {
		m.activeID = ""
		return Job{}, false
	}
	return *job, true
}

func (m *jobManager) cleanupLocked(now time.Time) {
	kept := make([]string, 0, len(m.order))
	for _, id := range m.order {
		job, exists := m.jobs[id]
		if !exists {
			continue
		}
		active := job.Status == jobQueued || job.Status == jobRunning
		expired := job.FinishedAt != nil && now.Sub(*job.FinishedAt) > jobRetention
		if !active && expired {
			delete(m.jobs, id)
			delete(m.controls, id)
			continue
		}
		kept = append(kept, id)
	}
	m.order = kept
	for len(m.order) > maxJobs {
		id := m.order[0]
		job := m.jobs[id]
		if job != nil && (job.Status == jobQueued || job.Status == jobRunning) {
			break
		}
		delete(m.jobs, id)
		delete(m.controls, id)
		m.order = m.order[1:]
	}
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}

func newJobID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("create job id: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
