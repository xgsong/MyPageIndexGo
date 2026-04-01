package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	MCPServerName    = "MyPageIndexGo"
	MCPServerVersion = "1.0.0"
)

func NewMCPServer() *server.MCPServer {
	s := server.NewMCPServer(MCPServerName, MCPServerVersion)
	registerTools(s)
	return s
}

func Run() error {
	s := NewMCPServer()
	return server.ServeStdio(s)
}

func registerTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool(
			"generate_index",
			mcp.WithDescription("从 PDF 或 Markdown 文档生成索引树。索引用于后续的语义搜索。"),
		),
		generateIndexHandler,
	)

	s.AddTool(
		mcp.NewTool(
			"search_index",
			mcp.WithDescription("在已生成的索引中搜索相关内容。使用 LLM 进行语义推理检索。"),
			mcp.WithString("index_path",
				mcp.Required(),
				mcp.Description("索引文件路径"),
			),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("搜索查询"),
			),
			mcp.WithString("output_path",
				mcp.Description("可选的输出文件路径，保存搜索结果为 JSON"),
			),
			mcp.WithString("model",
				mcp.Description("可选的 LLM 模型，覆盖 config.yaml 配置"),
			),
		),
		searchIndexHandler,
	)

	s.AddTool(
		mcp.NewTool(
			"update_index",
			mcp.WithDescription("向现有索引添加新文档内容。现有索引的节点会保留，新增文档会被合并。"),
		),
		updateIndexHandler,
	)
}
