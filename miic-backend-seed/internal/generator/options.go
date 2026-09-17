package generator

import "context"

type optionsKey struct{}
type generationOptions struct {
	OutputPath string
	Grayscale  bool
}
type progressKey struct{}

type Progress struct {
	Stage            string `json:"stage"`
	Processed        int    `json:"processed"`
	Total            int    `json:"total"`
	CurrentColumnID  int64  `json:"current_column_id,omitempty"`
	CurrentArticleID int64  `json:"current_article_id,omitempty"`
	GeneratedFiles   int    `json:"generated_files"`
}

func WithOptions(ctx context.Context, outputPath string, grayscale bool) context.Context {
	return context.WithValue(ctx, optionsKey{}, generationOptions{OutputPath: outputPath, Grayscale: grayscale})
}

func optionsFrom(ctx context.Context) generationOptions {
	value, _ := ctx.Value(optionsKey{}).(generationOptions)
	return value
}

func WithProgress(ctx context.Context, callback func(Progress)) context.Context {
	return context.WithValue(ctx, progressKey{}, callback)
}

func reportProgress(ctx context.Context, progress Progress) {
	if callback, ok := ctx.Value(progressKey{}).(func(Progress)); ok && callback != nil {
		callback(progress)
	}
}
