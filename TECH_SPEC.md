# PageIndex 迁移至Go技术栈 - 技术规格

## 项目概述

**PageIndex** 是一个基于LLM推理的无向量RAG（Retrieval-Augmented Generation）系统。

核心特点：
- 不需要向量数据库 / 文本分块 / embedding
- 通过LLM分析文档自动生成层次化目录树结构索引
- 基于目录树进行推理式检索，相比传统向量搜索准确率更高
- 在FinanceBench数据集上达到98.7%准确率

## 技术选型

| 功能模块 | 库选择 | 理由 |
|---------|--------|------|
| PDF处理 | [`github.com/ledongthuc/pdf`](https://github.com/ledongthuc/pdf) | 纯Go实现，无需CGO，专注于文本提取，API简洁 |
| Markdown处理 | [`github.com/yuin/goldmark`](https://github.com/yuin/goldmark) | 最流行的Go Markdown处理器，扩展性好 |
| OpenAI API | [`github.com/sashabaranov/go-openai`](https://github.com/sashabaranov/go-openai) | 社区标准实现，维护活跃，功能完整 |
| 令牌计数 | [`github.com/pkoukk/tiktoken-go`](https://github.com/pkoukk/tiktoken-go) | OpenAI tiktoken的Go移植，计数精度一致 |
| 配置管理 | [`github.com/spf13/viper`](https://github.com/spf13/viper) + [`github.com/joho/godotenv`](https://github.com/joho/godotenv) | 支持.env文件、环境变量、配置文件 |
| 命令行 | [`github.com/urfave/cli/v2`](https://github.com/urfave/cli) | 简洁强大的Go命令行库 |
| 并发控制 | `golang.org/x/sync/errgroup` | 原生goroutine并发，更好的错误传播 |

## 项目结构

```
mypageindexgo/
├── cmd/                    # 命令行入口
│   └── pageindex/
│       └── main.go         # 主入口
├── pkg/
│   ├── config/             # 配置处理
│   │   └── config.go
│   ├── document/           # 文档处理核心
│   │   ├── parser.go       # 解析器接口定义
│   │   ├── pdf.go          # PDF文档解析
│   │   ├── markdown.go     # Markdown文档解析
│   │   └── tree.go         # 目录树数据结构
│   ├── llm/                # LLM调用封装
│   │   ├── client.go       # LLM客户端接口
│   │   ├── openai.go       # OpenAI实现
│   │   └── prompts.go      # Prompt模板
│   ├── tokenizer/          # 令牌计数
│   │   └── tokenizer.go
│   ├── indexer/            # 索引生成与检索
│   │   ├── generator.go    # 目录树生成
│   │   ├── processor.go    # 节点处理
│   │   └── search.go       # 推理检索
│   └── output/             # 输出处理
│       └── json.go
├── internal/               # 内部私有工具
│   └── utils/
│       ├── json.go         # JSON工具
│       └── errors.go       # 错误处理
├── test/                   # 测试
│   └── fixtures/           # 测试文档
├── go.mod
├── go.sum
├── README.md
├── LICENSE
└── TECH_SPEC.md            # 本文件
```

## 设计模式：适配器模式支持多格式

为了支持未来扩展更多文件格式（DOCX, HTML, TXT等），文档解析采用**适配器模式**：

- 每个文件格式实现一个`DocumentParser`适配器
- 所有适配器输出统一的`Document`结构
- 下游索引流程无需修改即可支持新格式

### 核心数据结构

### 文档

```go
// DocumentParser 是适配器接口，每个文件格式实现一个适配器
// 适配器将输入格式转换为统一的Document输出
type DocumentParser interface {
    // Parse 解析输入文档为统一Document结构
    Parse(r io.Reader) (*Document, error)
    // SupportedExtensions 返回支持的文件扩展名列表（小写，无前缀点）
    SupportedExtensions() []string
    // Name 返回解析器名称，用于调试
    Name() string
}

// Document 表示解析后的统一文档
type Document struct {
    Pages    []Page          `json:"pages"`
    Metadata map[string]string `json:"metadata,omitempty"`
}

// Page 表示文档单个页面/分段
type Page struct {
    Number int    `json:"number"`
    Text   string `json:"text"`
}

// ParserRegistry 按文件扩展名维护解析器注册表
type ParserRegistry struct {
    parsers map[string]DocumentParser
}
```

### 目录树

```go
// Node 表示目录树节点
type Node struct {
    ID        string  `json:"id"`
    Title     string  `json:"title"`
    StartPage int     `json:"start_page"`
    EndPage   int     `json:"end_page"`
    Summary   string  `json:"summary,omitempty"`
    Children  []*Node `json:"children,omitempty"`
}

// IndexTree 完整文档索引树
type IndexTree struct {
    Root         *Node       `json:"root"`
    TotalPages   int         `json:"total_pages"`
    DocumentInfo string      `json:"document_info"`
    GeneratedAt  time.Time   `json:"generated_at"`
}

// SearchResult 检索结果
type SearchResult struct {
    Query    string  `json:"query"`
    Answer   string  `json:"answer"`
    Nodes    []*Node `json:"nodes"`
}
```

### LLM客户端

```go
// LLMClient 定义LLM客户端接口
// 支持未来扩展到其他LLM提供商
type LLMClient interface {
    GenerateStructure(ctx context.Context, text string) (*Node, error)
    GenerateSummary(ctx context.Context, text string) (string, error)
    Search(ctx context.Context, query string, tree *IndexTree) (*SearchResult, error)
}
```

## 并发模型

Python版本使用 `asyncio + ThreadPoolExecutor` 处理并发LLM调用。
Go版本使用 `goroutine + errgroup.Group`：

```go
// 控制最大并发数，避免API限流
group, ctx := errgroup.WithContext(ctx)
group.SetLimit(cfg.MaxConcurrency)

for _, pageGroup := range pageGroups {
    pageGroup := pageGroup
    group.Go(func() error {
        node, err := llmClient.GenerateStructure(ctx, pageGroup.Text)
        if err != nil {
            return fmt.Errorf("failed to generate structure: %w", err)
        }
        // 合并节点到结果树
        return nil
    })
}

if err := group.Wait(); err != nil {
    return nil, err
}
```

**优势**：
- goroutine比asyncio更轻量（KB级 vs MB级栈）
- errgroup提供原生错误传播
- 原生支持并发限制

## 错误处理

遵循Go惯用法：
- 显式返回 `(result, error)`
- 使用 `fmt.Errorf("context: %w", err)` 包装错误
- 在命令行顶层处理错误，友好显示
- 不静默吞掉错误

## 迁移阶段

### 阶段 1 - 基础项目搭建 ✓
- [x] 创建项目目录
- [x] Go模块初始化
- [x] Git初始化
- [x] 创建TECH_SPEC.md
- [x] 创建CLAUDE.md项目指南
- [x] 单元测试框架准备（添加testify）

### 阶段 2 - 核心文档解析模块 ✓
- [x] 创建目录结构
- [x] 添加依赖
- [x] PDF解析模块（使用github.com/ledongthuc/pdf）
- [x] Markdown解析模块（使用github.com/yuin/goldmark）
- [x] 令牌计数功能（使用github.com/pkoukk/tiktoken-go）
- [x] 配置管理模块（支持.env文件，OPENAI_API_KEY/OPENAI_BASE_URL）
- [x] 内部JSON工具（处理LLM输出非标准JSON）
- [x] 目录树数据结构定义
- [x] **单元测试**：每个模块都有完整单元测试
- [x] **架构调整**：适配器模式设计，预留扩展空间支持更多文件格式

### 需求变更记录

**2026-03-20**:
1. **环境变量配置**：OPENAI_API_KEY 和 OPENAI_BASE_URL 通过 `.env` 文件读取，同时保持 `PAGEINDEX_` 前缀向后兼容
2. **多格式支持**：采用适配器模式重构文档解析，不同格式转换为统一Document结构，便于后续扩展更多格式（DOCX, HTML等）
3. **LLM调用模块**：完成 `LLMClient` 接口抽象 + OpenAI实现，包含三个原始PageIndex提示词模板，复用JSONCleaner处理LLM输出，接口设计易于添加其他LLM提供商
4. **索引生成和检索模块**：完成完整索引生成流水线（页面分组 → 并行结构生成 → 合并树 → 摘要生成）和推理检索。使用 `goroutine + errgroup.Group` 实现并发控制和限流，支持配置 `GenerateSummaries` 开关。添加了 `MaxTokensPerNode` 默认值从 20000 调整为 16000 以预留足够prompt空间。
5. **命令行界面**：完成基于 `urfave/cli/v2` 的CLI实现，支持 `generate` 和 `search` 两个子命令。`generate` 从PDF/Markdown生成索引JSON，`search` 对生成的索引进行推理检索。配置优先级：CLI标志 > 环境变量 > 默认值。完成 `pkg/output` 模块提供JSON保存和加载功能。
6. **发布工程**：完成 `Makefile` 支持多平台交叉编译，添加 GitHub Actions CI 工作流自动测试和构建发布包，添加 README.md 说明文档，更新 `.env.example` 包含所有配置选项。项目完整可发布。

### 阶段 3 - LLM调用模块 ✓
- [x] OpenAI客户端封装（兼容OPENAI_BASE_URL）
- [x] Prompt模板移植（三个提示词：GenerateStructure, GenerateSummary, Search）
- [x] JSON解析与清理（复用internal/utils JSONCleaner）
- [x] 接口抽象设计，支持多LLM提供商扩展
- [x] **单元测试**：每个模块都有完整单元测试

### 阶段 4 - 索引生成和检索 ✓
- [x] 目录树生成算法
- [x] 节点分组处理（基于token计数）
- [x] 摘要生成
- [x] 推理检索
- [x] **单元测试**：每个模块都需要单元测试

### 阶段 5 - 命令行和输出 ✓
- [x] CLI参数解析（urfave/cli/v2）
- [x] 配置加载（支持.env环境变量）
- [x] JSON输出保存
- [x] **单元测试**：输出模块完整单元测试

### 阶段 6 - 优化发布 ✓
- [x] 性能优化（无明显优化点，当前架构已优化）
- [x] 交叉编译配置（Makefile支持多平台编译）
- [x] GitHub Action配置（CI工作流，自动测试和发布）
- [x] **完整测试覆盖**：所有核心包都有单元测试

## 迁移优势

| 维度 | Python | Go |
|------|--------|-----|
| 分发 | 需要Python环境 + pip | 单个静态二进制，直接运行 |
| 内存占用 | 较高 | 更低 |
| 启动速度 | 较慢 | 快速启动 |
| 并发 | GIL限制 | 原生轻量goroutine |
| 跨编译 | 复杂 | 简单，一行命令 |
| 部署 | 需要依赖管理 | 下载即运行，零依赖 |

## 注意事项

1. **纯Go优先**：使用纯Go实现的文本提取库，避免CGO简化交叉编译。当前使用 `github.com/ledongthuc/pdf`。

2. **JSON鲁棒性**：Python版本有复杂的JSON修复逻辑处理LLM不规范输出，这部分需要完整移植。已在 `internal/utils/json.go` 实现JSON清理。

3. **限流控制**：通过errgroup.SetLimit合理控制并发，避免触发OpenAI API限流。

4. **令牌精度验证**：需要验证tiktoken-go和Python tiktoken计数结果一致性。

5. **环境配置**：配置优先从 `.env` 文件读取，支持 `OPENAI_API_KEY` 和 `OPENAI_BASE_URL` 环境变量名称，同时兼容 `PAGEINDEX_` 前缀格式。

6. **多格式扩展**：新增格式只需要实现 `DocumentParser` 接口，并在 `ParserRegistry` 注册，无需修改下游索引代码。
