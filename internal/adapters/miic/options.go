package miic

import (
	"context"

	"portal-static/internal/contracts"
)

type Progress = contracts.Progress

func WithOptions(ctx context.Context, outputPath string, grayscale bool) context.Context {
	return contracts.WithOptions(ctx, outputPath, grayscale)
}

func optionsFrom(ctx context.Context) contracts.Options {
	return contracts.OptionsFrom(ctx)
}

func WithProgress(ctx context.Context, callback func(Progress)) context.Context {
	return contracts.WithProgress(ctx, callback)
}

func reportProgress(ctx context.Context, progress Progress) {
	contracts.ReportProgress(ctx, progress)
}
