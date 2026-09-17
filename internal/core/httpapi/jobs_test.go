package httpapi

import (
	"context"
	"strings"
	"testing"
	"time"

	generator "portal-static/internal/adapters/miic"
)

func TestJobManagerIdleTimeoutInterruptsJobAndReleasesSlot(t *testing.T) {
	manager := newJobManager(context.Background(), 20*time.Millisecond, time.Second)
	job, err := manager.start("lists", func(ctx context.Context) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitForManagedJob(t, manager, job.ID)
	if finished.Status != jobInterrupted || !strings.Contains(finished.Error, "made no progress") {
		t.Fatalf("unexpected job: %#v", finished)
	}
	if _, active := manager.active(); active {
		t.Fatal("idle timed-out job still occupies the batch slot")
	}
}

func TestCanceledJobKeepsSlotUntilWorkerActuallyExits(t *testing.T) {
	manager := newJobManager(context.Background(), time.Second, 2*time.Second)
	release := make(chan struct{})
	job, err := manager.start("lists", func(context.Context) (any, error) {
		<-release
		return nil, context.Canceled
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists, signaled := manager.cancel(job.ID); !exists || !signaled {
		t.Fatal("active job was not canceled")
	}
	if _, err := manager.start("articles", func(context.Context) (any, error) { return nil, nil }); err == nil {
		t.Fatal("new batch job started before canceled worker exited")
	}
	close(release)
	waitForManagedJob(t, manager, job.ID)
}

func TestJobManagerRecoversPanicAndSetsFinalProgress(t *testing.T) {
	manager := newJobManager(context.Background(), time.Second, 2*time.Second)
	panicked, err := manager.start("site", func(context.Context) (any, error) { panic("test panic") })
	if err != nil {
		t.Fatal(err)
	}
	failed := waitForManagedJob(t, manager, panicked.ID)
	if failed.Status != jobFailed || !strings.Contains(failed.Error, "test panic") {
		t.Fatalf("unexpected panic job: %#v", failed)
	}

	succeeded, err := manager.start("site", func(context.Context) (any, error) {
		return generator.GenerationResult{GeneratedFiles: 9}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	completed := waitForManagedJob(t, manager, succeeded.ID)
	if completed.Status != jobSucceeded || completed.Progress.GeneratedFiles != 9 || completed.FinishedAt == nil {
		t.Fatalf("unexpected completed job: %#v", completed)
	}
}

func waitForManagedJob(t *testing.T, manager *jobManager, id string) Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, exists := manager.get(id)
		if !exists {
			t.Fatal("job disappeared")
		}
		if job.Status == jobSucceeded || job.Status == jobFailed || job.Status == jobInterrupted {
			return job
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}
