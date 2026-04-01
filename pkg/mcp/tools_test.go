package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func newStringPtr(s string) *string {
	return &s
}

func TestGenerateIndexHandler_MissingFilePath(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "generate_index",
			Arguments: map[string]any{
				"file_path": "",
			},
		},
	}

	result, err := generateIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "file_path 是必需参数")
}

func TestGenerateIndexHandler_FileNotFound(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "generate_index",
			Arguments: map[string]any{
				"file_path": "/nonexistent/file.pdf",
			},
		},
	}

	result, err := generateIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "文件不存在")
}

func TestGenerateIndexHandler_UnsupportedFormat(t *testing.T) {
	tmpFile := "/tmp/test_unsupported.txt"
	f, err := os.Create(tmpFile)
	assert.NoError(t, err)
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmpFile)
	}()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "generate_index",
			Arguments: map[string]any{
				"file_path": tmpFile,
			},
		},
	}

	result, err := generateIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "不支持的文件格式")
}

func TestGenerateIndexRequest_BindArguments(t *testing.T) {
	args := map[string]any{
		"file_path":          "/tmp/test.pdf",
		"file_type":          "pdf",
		"output_path":        "/tmp/output.json",
		"model":              "gpt-4o",
		"max_concurrency":    float64(10),
		"generate_summaries": true,
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq GenerateIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "/tmp/test.pdf", boundReq.FilePath)
	assert.Equal(t, "pdf", *boundReq.FileType)
	assert.Equal(t, "/tmp/output.json", *boundReq.OutputPath)
	assert.Equal(t, "gpt-4o", *boundReq.Model)
	assert.Equal(t, 10, *boundReq.MaxConcurrency)
	assert.Equal(t, true, *boundReq.GenerateSummaries)
}

func TestSearchIndexHandler_MissingIndexPath(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "search_index",
			Arguments: map[string]any{
				"index_path": "",
				"query":      "test query",
			},
		},
	}

	result, err := searchIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "index_path 是必需参数")
}

func TestSearchIndexHandler_MissingQuery(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "search_index",
			Arguments: map[string]any{
				"index_path": "/tmp/test.index.json",
				"query":      "",
			},
		},
	}

	result, err := searchIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "query 是必需参数")
}

func TestSearchIndexHandler_IndexFileNotFound(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "search_index",
			Arguments: map[string]any{
				"index_path": "/nonexistent/index.json",
				"query":      "test query",
			},
		},
	}

	result, err := searchIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "索引加载失败")
}

func TestUpdateIndexHandler_ConfigLoadFailure(t *testing.T) {
	tmpIndexFile := "/tmp/test_update_config.index.json"
	tmpNewFile := "/tmp/test_new.md"

	indexContent := `{
		"root": {"id": "root", "title": "Test", "start_page": 1, "end_page": 10, "children": []},
		"total_pages": 10,
		"document_info": "Test",
		"generated_at": "2024-01-01T00:00:00Z",
		"last_modified": "2024-01-01T00:00:00Z"
	}`
	err := os.WriteFile(tmpIndexFile, []byte(indexContent), 0644)
	assert.NoError(t, err)

	err = os.WriteFile(tmpNewFile, []byte("# Test"), 0644)
	assert.NoError(t, err)

	defer func() {
		_ = os.Remove(tmpIndexFile)
		_ = os.Remove(tmpNewFile)
	}()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "update_index",
			Arguments: map[string]any{
				"existing_index_path": tmpIndexFile,
				"new_file_path":       tmpNewFile,
			},
		},
	}

	result, err := updateIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "配置加载失败")
}

func TestMarshalResult_Success(t *testing.T) {
	data := GenerateIndexResponse{
		Success:   true,
		IndexPath: "/tmp/test.index.json",
		Stats: IndexStats{
			TotalPages:  10,
			TotalNodes:  5,
			TimeSeconds: 1.5,
		},
	}

	result, err := marshalResult(data)

	assert.NoError(t, err)
	assert.False(t, result.IsError)

	var unmarshaled GenerateIndexResponse
	textContent := result.Content[0].(mcp.TextContent)
	err = json.Unmarshal([]byte(textContent.Text), &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, data, unmarshaled)
}

func TestMarshalResult_Error(t *testing.T) {
	type BadStruct struct {
		Chan chan int `json:"chan"`
	}
	data := BadStruct{Chan: make(chan int)}
	result, err := marshalResult(data)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "结果序列化失败")
}

func TestGenerateIndexHandler_PartialParameters(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "generate_index",
			Arguments: map[string]any{
				"file_path": "/tmp/test.pdf",
			},
		},
	}

	var boundReq GenerateIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "/tmp/test.pdf", boundReq.FilePath)
	assert.Nil(t, boundReq.FileType)
	assert.Nil(t, boundReq.OutputPath)
	assert.Nil(t, boundReq.Model)
	assert.Nil(t, boundReq.MaxConcurrency)
	assert.Nil(t, boundReq.GenerateSummaries)
}

func TestGenerateIndexHandler_InvalidParameterTypes(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "generate_index",
			Arguments: map[string]any{
				"file_path":       "/tmp/test.pdf",
				"max_concurrency": "invalid",
			},
		},
	}

	var boundReq GenerateIndexRequest
	err := req.BindArguments(&boundReq)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal string")
}

func TestGenerateIndexRequest_BindArgumentsWithDefaults(t *testing.T) {
	args := map[string]any{
		"file_path": "/tmp/test.md",
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq GenerateIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "/tmp/test.md", boundReq.FilePath)
}

func TestGenerateIndexRequest_FileTypeExtensions(t *testing.T) {
	testCases := []struct {
		name       string
		filePath   string
		isPDF      bool
		isMarkdown bool
	}{
		{"PDF lowercase", "/tmp/test.pdf", true, false},
		{"PDF uppercase", "/tmp/test.PDF", true, false},
		{"Markdown md", "/tmp/test.md", false, true},
		{"Markdown markdown", "/tmp/test.markdown", false, true},
		{"Mixed case PDF", "/tmp/test.Pdf", false, false},
		{"Mixed case MD", "/tmp/test.Md", false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile, err := os.Create(tc.filePath)
			assert.NoError(t, err)
			defer func() {
				_ = tmpFile.Close()
				_ = os.Remove(tc.filePath)
			}()

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "generate_index",
					Arguments: map[string]any{
						"file_path": tc.filePath,
					},
				},
			}

			result, err := generateIndexHandler(context.Background(), req)

			assert.NoError(t, err)
			if tc.isPDF || tc.isMarkdown {
				assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "配置加载失败")
			} else {
				assert.True(t, result.IsError)
				assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "不支持的文件格式")
			}
		})
	}
}

func TestIndexStats_JsonSerialization(t *testing.T) {
	stats := IndexStats{
		TotalPages:  100,
		TotalNodes:  25,
		TimeSeconds: 45.67,
	}

	data, err := json.Marshal(stats)
	assert.NoError(t, err)

	var unmarshaled IndexStats
	err = json.Unmarshal(data, &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, stats, unmarshaled)
}

func TestMergeStats_JsonSerialization(t *testing.T) {
	stats := MergeStats{
		OriginalPages: 50,
		NewPages:      30,
		TotalPages:    80,
		TotalNodes:    20,
		TimeSeconds:   25.5,
	}

	data, err := json.Marshal(stats)
	assert.NoError(t, err)

	var unmarshaled MergeStats
	err = json.Unmarshal(data, &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, stats, unmarshaled)
}

func TestGenerateIndexResponse_JsonSerialization(t *testing.T) {
	response := GenerateIndexResponse{
		Success:   true,
		IndexPath: "/tmp/test.index.json",
		Stats: IndexStats{
			TotalPages:  50,
			TotalNodes:  10,
			TimeSeconds: 30.0,
		},
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	var unmarshaled GenerateIndexResponse
	err = json.Unmarshal(data, &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, response, unmarshaled)
}

func TestReferencedNode_JsonSerialization(t *testing.T) {
	node := ReferencedNode{
		ID:        "abc123",
		Title:     "第一章：引言",
		StartPage: 1,
		EndPage:   10,
	}

	data, err := json.Marshal(node)
	assert.NoError(t, err)

	var unmarshaled ReferencedNode
	err = json.Unmarshal(data, &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, node, unmarshaled)
}

func TestSearchIndexResponse_JsonSerialization(t *testing.T) {
	response := SearchIndexResponse{
		Success: true,
		Query:   "测试查询",
		Answer:  "这是搜索结果",
		ReferencedNodes: []ReferencedNode{
			{ID: "1", Title: "节点 1", StartPage: 1, EndPage: 5},
			{ID: "2", Title: "节点 2", StartPage: 6, EndPage: 10},
		},
		SearchTime: 2.5,
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	var unmarshaled SearchIndexResponse
	err = json.Unmarshal(data, &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, response, unmarshaled)
}

func TestUpdateIndexResponse_JsonSerialization(t *testing.T) {
	response := UpdateIndexResponse{
		Success:    true,
		OutputPath: "/tmp/merged.index.json",
		Stats: MergeStats{
			OriginalPages: 50,
			NewPages:      25,
			TotalPages:    75,
			TotalNodes:    15,
			TimeSeconds:   20.0,
		},
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	var unmarshaled UpdateIndexResponse
	err = json.Unmarshal(data, &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, response, unmarshaled)
}

func TestGenerateIndexHandler_ContextCancellation(t *testing.T) {
	tmpFile, err := os.Create("/tmp/test_ctx.pdf")
	assert.NoError(t, err)
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove("/tmp/test_ctx.pdf")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "generate_index",
			Arguments: map[string]any{
				"file_path": tmpFile.Name(),
			},
		},
	}

	result, _ := generateIndexHandler(ctx, req)

	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "配置加载失败")
}

func TestGenerateIndexHandler_ErrorMessagesAreChinese(t *testing.T) {
	testCases := []struct {
		name     string
		req      mcp.CallToolRequest
		expected string
	}{
		{
			name: "missing file path",
			req: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "generate_index",
					Arguments: map[string]any{
						"file_path": "",
					},
				},
			},
			expected: "file_path 是必需参数",
		},
		{
			name: "file not found",
			req: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "generate_index",
					Arguments: map[string]any{
						"file_path": "/nonexistent_abc123.pdf",
					},
				},
			},
			expected: "文件不存在",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := generateIndexHandler(context.Background(), tc.req)
			assert.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Content[0].(mcp.TextContent).Text, tc.expected)
		})
	}
}

func TestGenerateIndexRequest_EmptyString(t *testing.T) {
	args := map[string]any{
		"file_path": "",
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq GenerateIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "", boundReq.FilePath)
}

func TestSearchIndexRequest_BindArguments(t *testing.T) {
	args := map[string]any{
		"index_path": "/tmp/test.index.json",
		"query":      "测试查询",
		"model":      "gpt-4o",
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq SearchIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "/tmp/test.index.json", boundReq.IndexPath)
	assert.Equal(t, "测试查询", boundReq.Query)
	assert.Equal(t, "gpt-4o", *boundReq.Model)
}

func TestSearchIndexRequest_MissingRequiredFields(t *testing.T) {
	args := map[string]any{
		"index_path": "/tmp/test.index.json",
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq SearchIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "/tmp/test.index.json", boundReq.IndexPath)
	assert.Equal(t, "", boundReq.Query)
}

func TestUpdateIndexRequest_BindArguments(t *testing.T) {
	args := map[string]any{
		"existing_index_path": "/tmp/existing.index.json",
		"new_file_path":       "/tmp/new.pdf",
		"model":               "gpt-4o-mini",
		"max_concurrency":     float64(5),
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq UpdateIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "/tmp/existing.index.json", boundReq.ExistingIndexPath)
	assert.Equal(t, "/tmp/new.pdf", boundReq.NewFilePath)
	assert.Equal(t, "gpt-4o-mini", *boundReq.Model)
	assert.Equal(t, 5, *boundReq.MaxConcurrency)
}

func TestSearchIndexHandler_ErrorMessagesAreChinese(t *testing.T) {
	testCases := []struct {
		name     string
		req      mcp.CallToolRequest
		expected string
	}{
		{
			name: "missing index path",
			req: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "search_index",
					Arguments: map[string]any{
						"index_path": "",
						"query":      "test",
					},
				},
			},
			expected: "index_path 是必需参数",
		},
		{
			name: "missing query",
			req: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "search_index",
					Arguments: map[string]any{
						"index_path": "/tmp/test.index.json",
						"query":      "",
					},
				},
			},
			expected: "query 是必需参数",
		},
		{
			name: "index file not found",
			req: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "search_index",
					Arguments: map[string]any{
						"index_path": "/nonexistent_xyz123.index.json",
						"query":      "test",
					},
				},
			},
			expected: "索引加载失败",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := searchIndexHandler(context.Background(), tc.req)
			assert.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Content[0].(mcp.TextContent).Text, tc.expected)
		})
	}
}

func TestSearchIndexRequest_EmptyString(t *testing.T) {
	args := map[string]any{
		"index_path": "",
		"query":      "",
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq SearchIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "", boundReq.IndexPath)
	assert.Equal(t, "", boundReq.Query)
}

func TestSearchIndexHandler_InvalidParameterTypes(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "search_index",
			Arguments: map[string]any{
				"index_path": 123,
				"query":      "test",
			},
		},
	}

	var boundReq SearchIndexRequest
	err := req.BindArguments(&boundReq)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal number")
}

func TestSearchIndexRequest_AllParameters(t *testing.T) {
	args := map[string]any{
		"index_path":  "/tmp/test.index.json",
		"query":       "测试查询",
		"output_path": "/tmp/result.json",
		"model":       "gpt-4o",
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq SearchIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "/tmp/test.index.json", boundReq.IndexPath)
	assert.Equal(t, "测试查询", boundReq.Query)
	assert.Equal(t, "/tmp/result.json", *boundReq.OutputPath)
	assert.Equal(t, "gpt-4o", *boundReq.Model)
}

func TestUpdateIndexRequest_MissingRequiredFields(t *testing.T) {
	args := map[string]any{
		"existing_index_path": "",
		"new_file_path":       "",
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq UpdateIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "", boundReq.ExistingIndexPath)
	assert.Equal(t, "", boundReq.NewFilePath)
}

func TestUpdateIndexRequest_EmptyString(t *testing.T) {
	args := map[string]any{
		"existing_index_path": "",
		"new_file_path":       "",
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq UpdateIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "", boundReq.ExistingIndexPath)
	assert.Equal(t, "", boundReq.NewFilePath)
}

func TestUpdateIndexRequest_InvalidParameterTypes(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "update_index",
			Arguments: map[string]any{
				"existing_index_path": 123,
				"new_file_path":       456,
			},
		},
	}

	var boundReq UpdateIndexRequest
	err := req.BindArguments(&boundReq)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal number")
}

func TestUpdateIndexHandler_MissingExistingIndexPath(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "update_index",
			Arguments: map[string]any{
				"existing_index_path": "",
				"new_file_path":       "/tmp/test.pdf",
			},
		},
	}

	result, err := updateIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "existing_index_path 是必需参数")
}

func TestUpdateIndexHandler_MissingNewFilePath(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "update_index",
			Arguments: map[string]any{
				"existing_index_path": "/tmp/existing.index.json",
				"new_file_path":       "",
			},
		},
	}

	result, err := updateIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "new_file_path 是必需参数")
}

func TestUpdateIndexHandler_NonExistentIndexPath(t *testing.T) {
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "update_index",
			Arguments: map[string]any{
				"existing_index_path": "/nonexistent_index_xyz123.index.json",
				"new_file_path":       "/tmp/test_new.pdf",
			},
		},
	}

	result, err := updateIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "现有索引加载失败")
}

func TestUpdateIndexHandler_NonExistentNewFile(t *testing.T) {
	tmpIndexFile := "/tmp/test_update_existing.index.json"
	indexContent := `{
		"root": {"id": "root", "title": "Test", "start_page": 1, "end_page": 10, "children": []},
		"total_pages": 10,
		"document_info": "Test",
		"generated_at": "2024-01-01T00:00:00Z",
		"last_modified": "2024-01-01T00:00:00Z"
	}`
	err := os.WriteFile(tmpIndexFile, []byte(indexContent), 0644)
	assert.NoError(t, err)
	defer func() {
		_ = os.Remove(tmpIndexFile)
	}()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "update_index",
			Arguments: map[string]any{
				"existing_index_path": tmpIndexFile,
				"new_file_path":       "/nonexistent_newfile_abc123.pdf",
			},
		},
	}

	result, err := updateIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "新文档文件不存在")
}

func TestUpdateIndexHandler_UnsupportedFormat(t *testing.T) {
	tmpIndexFile := "/tmp/test_update_fmt.index.json"
	tmpNewFile := "/tmp/test_unsupported.txt"

	indexContent := `{
		"root": {"id": "root", "title": "Test", "start_page": 1, "end_page": 10, "children": []},
		"total_pages": 10,
		"document_info": "Test",
		"generated_at": "2024-01-01T00:00:00Z",
		"last_modified": "2024-01-01T00:00:00Z"
	}`
	err := os.WriteFile(tmpIndexFile, []byte(indexContent), 0644)
	assert.NoError(t, err)

	f, err := os.Create(tmpNewFile)
	assert.NoError(t, err)
	defer func() {
		_ = f.Close()
		_ = os.Remove(tmpIndexFile)
		_ = os.Remove(tmpNewFile)
	}()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "update_index",
			Arguments: map[string]any{
				"existing_index_path": tmpIndexFile,
				"new_file_path":       tmpNewFile,
			},
		},
	}

	result, err := updateIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "不支持的文件格式")
}

func TestUpdateIndexRequest_PartialParameters(t *testing.T) {
	args := map[string]any{
		"existing_index_path": "/tmp/existing.index.json",
		"new_file_path":       "/tmp/new.pdf",
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq UpdateIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, "/tmp/existing.index.json", boundReq.ExistingIndexPath)
	assert.Equal(t, "/tmp/new.pdf", boundReq.NewFilePath)
	assert.Nil(t, boundReq.OutputPath)
	assert.Nil(t, boundReq.Model)
	assert.Nil(t, boundReq.MaxConcurrency)
}

func TestSearchIndexRequest_FloatModel(t *testing.T) {
	args := map[string]any{
		"index_path": "/tmp/test.index.json",
		"query":      "test",
		"model":      123.45,
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq SearchIndexRequest
	err := req.BindArguments(&boundReq)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal number")
}

func TestGenerateIndexRequest_NumberParameter(t *testing.T) {
	args := map[string]any{
		"file_path":          "/tmp/test.pdf",
		"max_concurrency":    float64(20),
		"generate_summaries": false,
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	var boundReq GenerateIndexRequest
	err := req.BindArguments(&boundReq)

	assert.NoError(t, err)
	assert.Equal(t, 20, *boundReq.MaxConcurrency)
	assert.Equal(t, false, *boundReq.GenerateSummaries)
}

func TestReferencedNode_EmptyFields(t *testing.T) {
	node := ReferencedNode{}

	data, err := json.Marshal(node)
	assert.NoError(t, err)

	var unmarshaled ReferencedNode
	err = json.Unmarshal(data, &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, "", unmarshaled.ID)
	assert.Equal(t, "", unmarshaled.Title)
	assert.Equal(t, 0, unmarshaled.StartPage)
	assert.Equal(t, 0, unmarshaled.EndPage)
}

func TestSearchIndexResponse_EmptyReferencedNodes(t *testing.T) {
	response := SearchIndexResponse{
		Success:         true,
		Query:           "test query",
		Answer:          "test answer",
		ReferencedNodes: []ReferencedNode{},
		SearchTime:      1.0,
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	var unmarshaled SearchIndexResponse
	err = json.Unmarshal(data, &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, response, unmarshaled)
	assert.Empty(t, unmarshaled.ReferencedNodes)
}

func TestMergeStats_ZeroValues(t *testing.T) {
	stats := MergeStats{}

	data, err := json.Marshal(stats)
	assert.NoError(t, err)

	var unmarshaled MergeStats
	err = json.Unmarshal(data, &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, stats, unmarshaled)
	assert.Equal(t, 0, unmarshaled.OriginalPages)
	assert.Equal(t, 0, unmarshaled.NewPages)
	assert.Equal(t, 0.0, unmarshaled.TimeSeconds)
}

func TestIndexStats_ZeroValues(t *testing.T) {
	stats := IndexStats{}

	data, err := json.Marshal(stats)
	assert.NoError(t, err)

	var unmarshaled IndexStats
	err = json.Unmarshal(data, &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, stats, unmarshaled)
}

func TestGenerateIndexResponse_EmptyIndexPath(t *testing.T) {
	response := GenerateIndexResponse{
		Success:   true,
		IndexPath: "",
		Stats: IndexStats{
			TotalPages:  0,
			TotalNodes:  0,
			TimeSeconds: 0.0,
		},
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	var unmarshaled GenerateIndexResponse
	err = json.Unmarshal(data, &unmarshaled)

	assert.NoError(t, err)
	assert.Equal(t, response, unmarshaled)
}

func TestSearchIndexHandler_WithValidIndexFile(t *testing.T) {
	tmpIndexFile := "/tmp/test_valid_index_file.index.json"
	tmpResultFile := "/tmp/test_search_result_file.json"

	indexContent := `{
		"root": {
			"id": "test-root",
			"title": "Test Document",
			"start_page": 1,
			"end_page": 10,
			"children": []
		},
		"total_pages": 10,
		"document_info": "测试文档",
		"generated_at": "2024-01-01T00:00:00Z",
		"last_modified": "2024-01-01T00:00:00Z"
	}`

	err := os.WriteFile(tmpIndexFile, []byte(indexContent), 0644)
	assert.NoError(t, err)
	defer func() {
		_ = os.Remove(tmpIndexFile)
		_ = os.Remove(tmpResultFile)
	}()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "search_index",
			Arguments: map[string]any{
				"index_path":  tmpIndexFile,
				"query":       "测试查询",
				"output_path": tmpResultFile,
			},
		},
	}

	result, err := searchIndexHandler(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "配置加载失败")
}

func TestSearchIndexHandler_ContextCancellation(t *testing.T) {
	tmpIndexFile := "/tmp/test_valid_index_ctx_cancel.index.json"
	tmpResultFile := "/tmp/test_context_cancel_result.json"

	indexContent := `{
		"root": {
			"id": "test-root",
			"title": "Test Document",
			"start_page": 1,
			"end_page": 10,
			"children": []
		},
		"total_pages": 10,
		"document_info": "测试文档",
		"generated_at": "2024-01-01T00:00:00Z",
		"last_modified": "2024-01-01T00:00:00Z"
	}`

	err := os.WriteFile(tmpIndexFile, []byte(indexContent), 0644)
	assert.NoError(t, err)
	defer func() {
		_ = os.Remove(tmpIndexFile)
		_ = os.Remove(tmpResultFile)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "search_index",
			Arguments: map[string]any{
				"index_path":  tmpIndexFile,
				"query":       "test query",
				"output_path": tmpResultFile,
			},
		},
	}

	result, err := searchIndexHandler(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "配置加载失败")
}
