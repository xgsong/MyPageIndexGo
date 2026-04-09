package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/indexer"
	"github.com/xgsong/mypageindexgo/pkg/llm"
	"github.com/xgsong/mypageindexgo/pkg/output"
	"github.com/xgsong/mypageindexgo/pkg/workflow"
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

	// Create document service for unified workflow
	svc, err := workflow.NewDocumentService(cfg)
	if err != nil {
		return mcp.NewToolResultErrorf("文档服务创建失败：%v", err), nil
	}

	// Progress callback for MCP notifications
	progressCb := func(done, total int, desc string) {
		srv := server.ServerFromContext(ctx)
		if srv != nil {
			_ = srv.SendNotificationToClient(ctx, "notifications/progress", map[string]any{
				"progressToken": request.Params.Meta.ProgressToken,
				"progress":      done,
				"total":         total,
				"message":       desc,
			})
		}
	}

	// Process document using unified workflow
	opts := workflow.DocumentServiceOptions{
		ProgressCallback: progressCb,
	}
	tree, err := svc.ProcessDocument(ctx, req.FilePath, opts)
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
	startTime := time.Now()

	var req SearchIndexRequest
	if err := request.BindArguments(&req); err != nil {
		return mcp.NewToolResultErrorf("参数解析失败：%v", err), nil
	}

	if req.IndexPath == "" {
		return mcp.NewToolResultError("index_path 是必需参数"), nil
	}

	if req.Query == "" {
		return mcp.NewToolResultError("query 是必需参数"), nil
	}

	sendProgress := func(done, total int, desc string) {
		srv := server.ServerFromContext(ctx)
		if srv != nil {
			_ = srv.SendNotificationToClient(ctx, "notifications/progress", map[string]any{
				"progressToken": request.Params.Meta.ProgressToken,
				"progress":      done,
				"total":         total,
				"message":       desc,
			})
		}
	}

	sendProgress(1, 3, "Index loaded, starting search")

	tree, err := output.LoadIndexTree(req.IndexPath)
	if err != nil {
		return mcp.NewToolResultErrorf("索引加载失败：%v", err), nil
	}

	sendProgress(1, 3, "Index loaded, starting search")

	cfg, err := config.Load()
	if err != nil {
		return mcp.NewToolResultErrorf("配置加载失败：%v", err), nil
	}

	if req.Model != nil && *req.Model != "" {
		cfg.OpenAIModel = *req.Model
	}

	var llmClient llm.LLMClient = llm.NewOpenAIClient(cfg)
	var searcher *indexer.Searcher
	if cfg.EnableLLMCache {
		cacheTTL := time.Duration(cfg.LLMCacheTTL) * time.Second
		llmClient = llm.NewCachedLLMClient(llmClient, cacheTTL, true)
	}
	searcher = indexer.NewSearcher(llmClient)

	sendProgress(2, 3, "Searching with LLM")

	result, err := searcher.Search(ctx, req.Query, tree)
	if err != nil {
		return mcp.NewToolResultErrorf("搜索失败：%v", err), nil
	}

	if req.OutputPath != nil && *req.OutputPath != "" {
		if err := output.SaveSearchResult(result, *req.OutputPath); err != nil {
			return mcp.NewToolResultErrorf("结果保存失败：%v", err), nil
		}
	}

	referencedNodes := make([]ReferencedNode, 0, len(result.Nodes))
	for _, node := range result.Nodes {
		referencedNodes = append(referencedNodes, ReferencedNode{
			ID:        node.ID,
			Title:     node.Title,
			StartPage: node.StartPage,
			EndPage:   node.EndPage,
		})
	}

	sendProgress(3, 3, "Search complete")

	response := SearchIndexResponse{
		Success:         true,
		Query:           result.Query,
		Answer:          result.Answer,
		ReferencedNodes: referencedNodes,
		SearchTime:      time.Since(startTime).Seconds(),
	}

	return marshalResult(response)
}

func updateIndexHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	startTime := time.Now()

	var req UpdateIndexRequest
	if err := request.BindArguments(&req); err != nil {
		return mcp.NewToolResultErrorf("参数解析失败：%v", err), nil
	}

	if req.ExistingIndexPath == "" {
		return mcp.NewToolResultError("existing_index_path 是必需参数"), nil
	}

	if req.NewFilePath == "" {
		return mcp.NewToolResultError("new_file_path 是必需参数"), nil
	}

	sendProgress := func(done, total int, desc string) {
		srv := server.ServerFromContext(ctx)
		if srv != nil {
			_ = srv.SendNotificationToClient(ctx, "notifications/progress", map[string]any{
				"progressToken": request.Params.Meta.ProgressToken,
				"progress":      done,
				"total":         total,
				"message":       desc,
			})
		}
	}

	sendProgress(1, 6, "Loading existing index")

	existingTree, err := output.LoadIndexTree(req.ExistingIndexPath)
	if err != nil {
		return mcp.NewToolResultErrorf("现有索引加载失败：%v", err), nil
	}

	sendProgress(2, 6, "Loading configuration")

	cfg, err := config.Load()
	if err != nil {
		return mcp.NewToolResultErrorf("配置加载失败：%v", err), nil
	}

	if req.Model != nil && *req.Model != "" {
		cfg.OpenAIModel = *req.Model
	}

	if req.MaxConcurrency != nil {
		cfg.MaxConcurrency = *req.MaxConcurrency
	}

	// Create document service for unified workflow
	svc, err := workflow.NewDocumentService(cfg)
	if err != nil {
		return mcp.NewToolResultErrorf("文档服务创建失败：%v", err), nil
	}

	sendProgress(3, 6, "Parsing new document")

	// Parse new document using the service
	newDoc, err := svc.ParseDocument(ctx, req.NewFilePath)
	if err != nil {
		return mcp.NewToolResultErrorf("文档解析失败：%v", err), nil
	}

	sendProgress(4, 6, "Generating index for new document")

	// Create index generator using the service's LLM client
	generator, err := indexer.NewIndexGenerator(cfg, svc.LLMClient())
	if err != nil {
		return mcp.NewToolResultErrorf("索引生成器创建失败：%v", err), nil
	}

	mergedTree, err := generator.Update(ctx, existingTree, newDoc)
	if err != nil {
		return mcp.NewToolResultErrorf("索引更新失败：%v", err), nil
	}

	sendProgress(5, 6, "Saving merged index")

	outputPath := req.OutputPath
	if outputPath == nil || *outputPath == "" {
		defaultPath := req.ExistingIndexPath + ".merged.json"
		outputPath = &defaultPath
	}

	if err := output.SaveIndexTree(mergedTree, *outputPath); err != nil {
		return mcp.NewToolResultErrorf("索引保存失败：%v", err), nil
	}

	response := UpdateIndexResponse{
		Success:    true,
		OutputPath: *outputPath,
		Stats: MergeStats{
			OriginalPages: existingTree.TotalPages,
			NewPages:      len(newDoc.Pages),
			TotalPages:    mergedTree.TotalPages,
			TotalNodes:    mergedTree.CountAllNodes(),
			TimeSeconds:   time.Since(startTime).Seconds(),
		},
	}

	sendProgress(6, 6, "Update complete")

	return marshalResult(response)
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
