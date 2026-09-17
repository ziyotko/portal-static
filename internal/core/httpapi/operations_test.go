package httpapi

import (
	"context"
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
