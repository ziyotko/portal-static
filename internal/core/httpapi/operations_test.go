package httpapi

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func completeOperations() Operations {
	operation := func(context.Context) (any, error) { return nil, nil }
	article := func(context.Context, int64) (any, error) { return nil, nil }
	return Operations{
		GenerateSite: operation, GeneratePages: operation, GenerateAllLists: operation, GenerateAllArticles: operation,
		GeneratePage: func(context.Context, string) (any, error) { return nil, nil },
		GenerateList: article, GenerateListByName: func(context.Context, string) (any, error) { return nil, nil },
		GenerateArticle: article, DeleteArticle: article, GenerateArticleRelated: article, DeleteArticleRelated: article,
		NormalizePageName: func(name string) (string, bool) { return name, true },
		PageNameError:     "invalid page", ValidateOutputPath: func(string) error { return nil },
		ClassifyError: func(error) (int, string, bool) { return 0, "", false },
	}
}

func TestOperationsValidateRequiresCompletePlatformContract(t *testing.T) {
	operations := completeOperations()
	if err := operations.Validate(); err != nil {
		t.Fatal(err)
	}
	operations.DeleteArticleRelated = nil
	if err := operations.Validate(); err == nil || !strings.Contains(err.Error(), "DeleteArticleRelated") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestReloadingOperationsUsesFreshRuntimeAndCleansUp(t *testing.T) {
	loads := 0
	cleanups := 0
	base := completeOperations()
	reloading := ReloadingOperations(base, func(context.Context) (Operations, func() error, error) {
		loads++
		current := completeOperations()
		version := loads
		current.GeneratePage = func(context.Context, string) (any, error) { return version, nil }
		return current, func() error {
			cleanups++
			return nil
		}, nil
	})

	first, err := reloading.GeneratePage(context.Background(), "home")
	if err != nil || first != 1 {
		t.Fatalf("first result = %v, %v", first, err)
	}
	second, err := reloading.GeneratePage(context.Background(), "home")
	if err != nil || second != 2 {
		t.Fatalf("second result = %v, %v", second, err)
	}
	if loads != 2 || cleanups != 2 {
		t.Fatalf("loads=%d cleanups=%d", loads, cleanups)
	}
}

func TestReloadingOperationsJoinsCleanupError(t *testing.T) {
	base := completeOperations()
	reloading := ReloadingOperations(base, func(context.Context) (Operations, func() error, error) {
		current := completeOperations()
		current.GenerateSite = func(context.Context) (any, error) { return nil, errors.New("generate failed") }
		return current, func() error { return errors.New("cleanup failed") }, nil
	})

	_, err := reloading.GenerateSite(context.Background())
	if err == nil || !strings.Contains(err.Error(), "generate failed") || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("unexpected joined error: %v", err)
	}
}
