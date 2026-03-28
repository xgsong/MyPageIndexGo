# MyPageIndexGo MCP Server 技术设计

## 1. 概述

### 1.1 背景

MCP (Model Context Protocol) 是由 Anthropic 于 2024 年 11 月推出的开放协议，旨在标准化 AI 模型与外部工具的交互方式。MCP 被形象地称为 AI 应用的"USB-C 端口"——就像 USB-C 提供了一种连接设备的标准化方式，MCP 提供了一种连接 AI 模型与外部数据源/工具的标准化方式。

通过为 MyPageIndexGo 实现 MCP Server，第三方 Go 项目可以通过 MCP 协议方便地集成文档索引和搜索功能，无需直接依赖库代码。

### 1.2 目标

- 为 MyPageIndexGo 添加 MCP Server 支持
- 通过 stdio 传输协议暴露 `generate`、`search`、`update` 三个核心功能
- 遵循 MCP 协议规范，支持 Claude Desktop、Cursor、Cline 等 MCP 客户端

### 1.3 非目标

- 不实现 HTTP/SSE 传输协议（当前版本）
- 不实现认证机制（当前版本）
- 不独立部署为单独进程（内嵌模式）

## 2. 技术选型

### 2.1 MCP SDK

| 方案 | 库 | 优点 | 缺点 |
|------|-----|------|------|
| **推荐** | `github.com/mark3labs/mcp-go` | 纯 Go 实现，API 简洁，示例丰富 | 社区维护 |
| 备选 | `github.com/modelcontextprotocol/go-server` | 官方维护 | 资料较少 |

选择 `mark3labs/mcp-go` 的原因：
- 纯 Go 实现，无 CGO 依赖
- API 设计简洁易用
- 有可参考的示例代码

### 2.2 依赖添加

```go
require github.com/mark3labs/mcp-go v0.x.x
```

## 3. 架构设计

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│  MCP Client (Claude Desktop / Cursor / Cline / 自定义)       │
│  通过 stdio 连接                                              │
└─────────────────────────────────┬───────────────────────────┘
                                  │ stdio (JSON-RPC over stdin/stdout)
                                  ▼
┌─────────────────────────────────────────────────────────────┐
│  MyPageIndexGo MCP Server                                  │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  pkg/mcp/server.go                                    │ │
│  │  - MCPServer 实例初始化                                │ │
│  │  - stdio 传输层配置                                    │ │
│  │  - 工具注册                                            │ │
│  └───────────────────────────────────────────────────────┘ │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  pkg/mcp/tools.go                                     │ │
│  │  - generate_index_tool (生成索引)                     │ │
│  │  - search_index_tool (搜索索引)                       │ │
│  │  - update_index_tool (更新索引)                      │ │
│  └───────────────────────────────────────────────────────┘ │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  pkg/mcp/types.go                                    │ │
│  │  - 工具输入/输出类型定义                               │ │
│  │  - JSON Schema 定义                                   │ │
│  └───────────────────────────────────────────────────────┘ │
│                              │                              │
│                              ▼                              │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  复用现有 pkg                                         │ │
│  │  - indexer.IndexGenerator                            │ │
│  │  - indexer.Searcher                                   │ │
│  │  - document.Parser (PDF/Markdown)                     │ │
│  │  - output.SaveIndexTree / LoadIndexTree              │ │
│  │  - config.Load                                       │ │
│  └───────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 目录结构

```
pkg/
  └── mcp/
      ├── server.go      # MCP Server 主入口 (~80行)
      ├── tools.go       # 工具定义和实现 (~350行)
      ├── types.go       # 类型定义 (~100行)
      └── cmd/
          └── mcp/        # MCP 独立启动入口
              └── main.go # 独立启动 main (~60行)
```

### 3.3 工具定义

#### 3.3.1 generate_index

生成文档索引。

```json
{
  "name": "generate_index",
  "description": "从 PDF 或 Markdown 文档生成索引树。索引用于后续的语义搜索。",
  "inputSchema": {
    "type": "object",
    "properties": {
      "file_path": {
        "type": "string",
        "description": "文档文件路径 (支持 PDF 或 Markdown)"
      },
      "file_type": {
        "type": "string",
        "enum": ["pdf", "markdown"],
        "description": "文档类型，默认根据文件扩展名自动检测"
      },
      "output_path": {
        "type": "string",
        "description": "输出索引文件路径，默认 {file_path}.index.json"
      },
      "model": {
        "type": "string",
        "description": "使用的 LLM 模型，默认使用 config.yaml 配置"
      },
      "max_concurrency": {
        "type": "integer",
        "description": "最大并发 LLM 调用数，默认使用 config.yaml 配置"
      },
      "generate_summaries": {
        "type": "boolean",
        "description": "是否生成节点摘要，默认 false"
      }
    },
    "required": ["file_path"]
  }
}
```

**输出示例**：
```json
{
  "success": true,
  "index_path": "/path/to/document.index.json",
  "stats": {
    "total_pages": 50,
    "total_nodes": 12,
    "time_seconds": 30.5
  }
}
```

#### 3.3.2 search_index

在已有索引中搜索。

```json
{
  "name": "search_index",
  "description": "在已生成的索引中搜索相关内容。使用 LLM 进行语义推理检索。",
  "inputSchema": {
    "type": "object",
    "properties": {
      "index_path": {
        "type": "string",
        "description": "索引文件路径 (.index.json)"
      },
      "query": {
        "type": "string",
        "description": "搜索查询语句"
      },
      "output_path": {
        "type": "string",
        "description": "可选：搜索结果输出文件路径"
      },
      "model": {
        "type": "string",
        "description": "使用的 LLM 模型，默认使用 config.yaml 配置"
      }
    },
    "required": ["index_path", "query"]
  }
}
```

**输出示例**：
```json
{
  "success": true,
  "query": "2023年的总营收是多少？",
  "answer": "根据财务报告，2023年公司总营收为 1.2 亿元，同比增长 15%。",
  "referenced_nodes": [
    {
      "id": "a1b2c3d4e5f6",
      "title": "财务 Performance 摘要",
      "start_page": 15,
      "end_page": 16
    },
    {
      "id": "b2c3d4e5f6g7",
      "title": "营收分析",
      "start_page": 12,
      "end_page": 14
    }
  ],
  "search_time_seconds": 2.3
}
```

#### 3.3.3 update_index

增量更新索引。

```json
{
  "name": "update_index",
  "description": "向现有索引添加新文档内容。现有索引的节点会保留，新增文档会被合并。",
  "inputSchema": {
    "type": "object",
    "properties": {
      "existing_index_path": {
        "type": "string",
        "description": "现有索引文件路径"
      },
      "new_file_path": {
        "type": "string",
        "description": "新文档文件路径 (PDF 或 Markdown)"
      },
      "new_file_type": {
        "type": "string",
        "enum": ["pdf", "markdown"],
        "description": "新文档类型，默认根据文件扩展名自动检测"
      },
      "output_path": {
        "type": "string",
        "description": "输出索引文件路径，默认覆盖现有索引"
      },
      "model": {
        "type": "string",
        "description": "使用的 LLM 模型，默认使用 config.yaml 配置"
      },
      "max_concurrency": {
        "type": "integer",
        "description": "最大并发 LLM 调用数，默认使用 config.yaml 配置"
      }
    },
    "required": ["existing_index_path", "new_file_path"]
  }
}
```

**输出示例**：
```json
{
  "success": true,
  "output_path": "/path/to/merged.index.json",
  "stats": {
    "original_pages": 50,
    "new_pages": 20,
    "total_pages": 70,
    "total_nodes": 18,
    "time_seconds": 15.2
  }
}
```

## 4. 实现细节

### 4.1 Server 初始化 (server.go)

```go
package mcp

import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

func NewMCPServer() (*server.MCPServer, error) {
    s := server.NewMCPServer(
        "MyPageIndexGo",
        "1.0.0",
        server.WithCapabilities(
            &mcp.ServerCapabilities{
                ToolsProvider: true,
            },
        ),
    )

    // 注册工具
    if err := registerTools(s); err != nil {
        return nil, err
    }

    return s, nil
}

func Run() error {
    s, err := NewMCPServer()
    if err != nil {
        return err
    }

    // 使用 stdio 传输
    return server.NewStdioServer(s).Serve()
}
```

### 4.2 工具注册 (tools.go)

```go
func registerTools(s *server.MCPServer) error {
    // generate_index
    s.AddTool(
        mcp.NewTool(
            "generate_index",
            mcp.WithDescription("从 PDF 或 Markdown 文档生成索引树"),
            mcp.WithInputSchema(generateIndexInputSchema),
        ),
        generateIndexHandler,
    )

    // search_index
    s.AddTool(
        mcp.NewTool(
            "search_index",
            mcp.WithDescription("在已生成的索引中搜索相关内容"),
            mcp.WithInputSchema(searchIndexInputSchema),
        ),
        searchIndexHandler,
    )

    // update_index
    s.AddTool(
        mcp.NewTool(
            "update_index",
            mcp.WithDescription("向现有索引添加新文档内容"),
            mcp.WithInputSchema(updateIndexInputSchema),
        ),
        updateIndexHandler,
    )

    return nil
}
```

### 4.3 工具处理器 (tools.go)

#### generateIndexHandler 示例

```go
func generateIndexHandler(
    ctx context.Context,
    request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
    args := request.Params.Arguments

    // 解析参数
    filePath, _ := args["file_path"].(string)
    outputPath, _ := args["output_path"].(string)
    // ... 其他参数解析

    // 验证文件存在
    if _, err := os.Stat(filePath); err != nil {
        return mcp.NewCallToolResultError("文件不存在: " + filePath), nil
    }

    // 加载配置
    cfg, err := config.Load()
    if err != nil {
        return mcp.NewCallToolResultError("配置加载失败: " + err.Error()), nil
    }

    // 解析文档
    file, err := os.Open(filePath)
    if err != nil {
        return mcp.NewCallToolResultError("文件打开失败: " + err.Error()), nil
    }
    defer file.Close()

    var parser document.DocumentParser
    if isPDF(filePath) {
        parser = document.NewPDFParser()
    } else {
        parser = document.NewMarkdownParser()
    }

    doc, err := parser.Parse(file)
    if err != nil {
        return mcp.NewCallToolResultError("文档解析失败: " + err.Error()), nil
    }

    // 生成索引
    llmClient := llm.NewOpenAIClient(cfg)
    generator, err := indexer.NewIndexGenerator(cfg, llmClient)
    if err != nil {
        return mcp.NewCallToolResultError("索引生成器创建失败: " + err.Error()), nil
    }

    tree, err := generator.Generate(ctx, doc)
    if err != nil {
        return mcp.NewCallToolResultError("索引生成失败: " + err.Error()), nil
    }

    // 保存索引
    if outputPath == "" {
        outputPath = filePath + ".index.json"
    }
    if err := output.SaveIndexTree(tree, outputPath); err != nil {
        return mcp.NewCallToolResultError("索引保存失败: " + err.Error()), nil
    }

    // 返回结果
    return mcp.NewCallToolResultOK(map[string]interface{}{
        "success":      true,
        "index_path":   outputPath,
        "stats": map[string]interface{}{
            "total_pages":    tree.TotalPages,
            "total_nodes":    tree.CountAllNodes(),
            "time_seconds":   time.Since(startTime).Seconds(),
        },
    }), nil
}
```

## 5. 独立启动入口

提供独立的 MCP 启动入口：

```go
// cmd/mcp/main.go
package main

import (
    "log"
    "github.com/xgsong/mypageindexgo/pkg/mcp"
)

func main() {
    if err := mcp.Run(); err != nil {
        log.Fatal(err)
    }
}
```

构建：
```bash
go build -o pageindex-mcp ./cmd/mcp
```

运行：
```bash
./pageindex-mcp
# Server 通过 stdio 通信，等待 MCP Client 连接
```

## 6. MCP Client 配置

### Claude Desktop 配置

```json
{
  "mcpServers": {
    "pageindex": {
      "command": "/path/to/pageindex-mcp",
      "args": []
    }
  }
}
```

### Cursor 配置

在 `.cursor/mcp.json` 添加：

```json
{
  "mcpServers": {
    "pageindex": {
      "command": "/path/to/pageindex-mcp"
    }
  }
}
```

### Cline 配置

在 VS Code 设置中添加：

```json
{
  "cline.mcpServers": {
    "pageindex": {
      "command": "/path/to/pageindex-mcp"
    }
  }
}
```

## 7. 错误处理

### 7.1 错误响应格式

工具执行失败时，返回错误信息：

```go
return mcp.NewCallToolResultError("错误描述: " + err.Error()), nil
```

### 7.2 错误类型映射

| 场景 | 返回消息 |
|------|----------|
| 文件不存在 | "文件不存在: {path}" |
| 文件格式不支持 | "不支持的文件格式: {ext}" |
| 配置加载失败 | "配置加载失败: {detail}" |
| 索引加载失败 | "索引加载失败: {detail}" |
| LLM 调用失败 | "LLM 调用失败: {detail}" |
| 超时 | "操作超时，请重试" |

## 8. 测试策略

### 8.1 单元测试

测试各个工具处理器的逻辑：

```go
func TestGenerateIndexTool(t *testing.T) {
    // 测试参数验证
    // 测试文件不存在情况
    // 测试成功生成情况
}
```

### 8.2 集成测试

使用 mock MCP client 测试完整流程：

```go
func TestMCPServerIntegration(t *testing.T) {
    // 启动 MCP Server (subprocess)
    // 使用 MCP client 连接
    // 调用工具并验证结果
}
```

### 8.3 MCP 协议兼容性测试

验证 JSON-RPC 消息格式符合规范。

## 9. 限制与注意事项

### 9.1 当前限制

- 仅支持 stdio 传输协议
- 无认证机制（敏感操作需由客户端控制）
- 无流式响应支持（长时间操作会阻塞）

### 9.2 后续扩展方向

- 添加 HTTP + SSE 传输协议支持
- 添加 API Key 认证
- 添加流式响应支持（进度回调）
- 添加更多工具（如 `list_indexes`、`delete_index`）

## 10. 文件清单

| 文件 | 行数估算 | 职责 |
|------|----------|------|
| `pkg/mcp/server.go` | ~80 | MCP Server 初始化和运行 |
| `pkg/mcp/tools.go` | ~350 | 工具定义和处理器实现 |
| `pkg/mcp/types.go` | ~100 | 类型定义和 JSON Schema |
| `cmd/mcp/main.go` | ~60 | 独立启动入口 |
| **总计** | ~590 | |

符合项目规范：静态语言单个文件不超过 250 行。

## 11. 变更记录

| 日期 | 版本 | 描述 |
|------|------|------|
| 2026-03-29 | v1.0.0 | 初始版本 |
