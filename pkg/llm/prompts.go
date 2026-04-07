package llm

import (
	"github.com/bytedance/sonic"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/prompts"
)

func GenerateStructurePrompt() string {
	return prompts.GenerateStructurePrompt()
}

func RenderSummaryPrompt(nodeTitle, text string) (string, error) {
	return prompts.RenderSummaryPrompt(nodeTitle, text)
}

func SearchPrompt(query string, tree *document.IndexTree) (string, error) {
	treeJSON, err := sonic.MarshalIndent(tree, "", "  ")
	if err != nil {
		return "", err
	}
	return prompts.RenderSearchPrompt(query, string(treeJSON))
}

func batchSummaryPrompt() string {
	return prompts.BatchSummaryPrompt()
}

func RenderBatchSummaryPrompt(requests []*BatchSummaryRequest) (string, error) {
	requestsJSON, err := sonic.MarshalIndent(requests, "", "  ")
	if err != nil {
		return "", err
	}
	return batchSummaryPrompt() + string(requestsJSON), nil
}
