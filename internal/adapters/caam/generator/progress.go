package generator

import (
	"context"

	"portal-static/internal/contracts"
)

type Progress = contracts.Progress

func WithProgress(ctx context.Context, report func(Progress)) context.Context {
	if report == nil {
		return ctx
	}
	return contracts.WithProgress(ctx, report)
}

func reportProgress(ctx context.Context, progress Progress) {
	contracts.ReportProgress(ctx, progress)
}
