package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/indexer"
	"github.com/xgsong/mypageindexgo/pkg/llm"
	"github.com/xgsong/mypageindexgo/pkg/output"
)

func generateIndexHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	startTime := time.Now()

	var req GenerateIndexRequest
	if err := request.BindArguments(&req); err != nil {
		return mcp.NewToolResultErrorf("参数解析失败：%v", err), nil
	}

	if req.FilePath == "" {
		return mcp.NewToolResultError("file_path 是必需参数"), nil
	}

	if _, err := os.Stat(req.FilePath); err != nil {
		return mcp.NewToolResultErrorf("文件不存在：%s", req.FilePath), nil
	}

	ext := filepath.Ext(req.FilePath)
	if ext != ".pdf" && ext != ".PDF" && ext != ".md" && ext != ".markdown" {
		return mcp.NewToolResultErrorf("不支持的文件格式：%s", ext), nil
	}

	cfg, err := config.Load()
	if err != nil {
		return mcp.NewToolResultErrorf("配置加载失败：%v", err), nil
	}

	if req.Model != nil {
		cfg.OpenAIModel = *req.Model
	}

	if req.MaxConcurrency != nil {
		cfg.MaxConcurrency = *req.MaxConcurrency
	}

	if req.GenerateSummaries != nil {
		cfg.GenerateSummaries = *req.GenerateSummaries
	}

	file, err := os.Open(req.FilePath)
	if err != nil {
		return mcp.NewToolResultErrorf("文件打开失败：%v", err), nil
	}
	defer func() {
		_ = file.Close()
	}()

	var parser document.DocumentParser
	if ext == ".pdf" || ext == ".PDF" {
		parser = document.NewPDFParser()
	} else if ext == ".md" || ext == ".markdown" {
		parser = document.NewMarkdownParser()
	} else {
		return mcp.NewToolResultErrorf("不支持的文件格式：%s", ext), nil
	}

	doc, err := parser.Parse(file)
	if err != nil {
		return mcp.NewToolResultErrorf("文档解析失败：%v", err), nil
	}

	llmClient := llm.NewOpenAIClient(cfg)
	generator, err := indexer.NewIndexGenerator(cfg, llmClient)
	if err != nil {
		return mcp.NewToolResultErrorf("索引生成器创建失败：%v", err), nil
	}

	tree, err := generator.Generate(ctx, doc)
	if err != nil {
		return mcp.NewToolResultErrorf("索引生成失败：%v", err), nil
	}

	outputPath := req.OutputPath
	if outputPath == nil || *outputPath == "" {
		defaultPath := req.FilePath + ".index.json"
		outputPath = &defaultPath
	}

	if err := output.SaveIndexTree(tree, *outputPath); err != nil {
		return mcp.NewToolResultErrorf("索引保存失败：%v", err), nil
	}

	response := GenerateIndexResponse{
		Success:   true,
		IndexPath: *outputPath,
		Stats: IndexStats{
			TotalPages:  tree.TotalPages,
			TotalNodes:  tree.CountAllNodes(),
			TimeSeconds: time.Since(startTime).Seconds(),
		},
	}

	return marshalResult(response)
}

func searchIndexHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("search_index 工具尚未实现"), nil
}

func updateIndexHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("update_index 工具尚未实现"), nil
}

func marshalResult(data any) (*mcp.CallToolResult, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultErrorf("结果序列化失败：%v", err), nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}
