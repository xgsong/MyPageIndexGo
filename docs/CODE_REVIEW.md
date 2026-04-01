# MyPageIndexGo 代码审查报告

> 审查日期: 2026-04-01

## 一、项目概述

这是一个基于 Go 语言实现的 **向量无关、推理驱动的 RAG 系统**，核心功能是从 PDF/Markdown 文档生成结构化索引树，并支持语义搜索。项目同时提供 CLI 工具和 MCP Server 两种使用方式。

---

## 二、架构设计审查

### ✅ 优点

#### 1. 清晰的分层架构

项目采用标准的 Go 项目布局：

- `cmd/` - 入口程序
- `pkg/` - 公共包
- `internal/` - 内部工具

模块职责划分清晰，遵循单一职责原则。

#### 2. 良好的接口抽象

关键组件都通过接口解耦：

```go
// LLM 客户端接口
type LLMClient interface {
    GenerateStructure(ctx context.Context, text string, lang language.Language) (*document.Node, error)
    GenerateSummary(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error)
    Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error)
    // ...
}

// 文档解析器接口
type DocumentParser interface {
    Parse(r io.Reader) (*Document, error)
    SupportedExtensions() []string
    Name() string
}

// OCR 客户端接口
type OCRClient interface {
    Recognize(ctx context.Context, req *OCRRequest) (*OCRResponse, error)
    RecognizeBatch(ctx context.Context, reqs []*OCRRequest) ([]*OCRResponse, error)
}
```

这种设计支持未来扩展不同的 LLM 提供商和文档格式。

#### 3. 装饰器模式的应用

`CachedLLMClient` 使用装饰器模式为任意 LLM 客户端添加缓存能力：

```go
func NewCachedLLMClient(client LLMClient, ttl time.Duration, enableSearchCache bool) LLMClient {
    return &CachedLLMClient{
        llmClient:         client,
        cache:             NewLRUCache(DefaultMaxCacheEntries, ttl),
        // ...
    }
}
```

### ⚠️ 问题与建议

#### 1. **indexer 模块文件过多，存在"数据泥团"问题**

`pkg/indexer/` 目录包含 **22 个文件**，违反了"每个文件夹不超过 8 个文件"的规则。

**建议重构**：

```
pkg/indexer/
├── core/           # 核心生成逻辑
│   ├── generator.go
│   └── processor.go
├── toc/            # TOC 相关
│   ├── detector.go
│   ├── extraction.go
│   └── types.go
├── search/         # 搜索功能
│   └── search.go
└── ratelimit/      # 限流器
    └── rate_limiter.go
```

#### 2. **generator_simple.go 文件过长（758 行）**

`generator_simple.go` 超过了静态语言 250 行的限制。该文件包含多个职责：

- TOC 树生成
- 节点合并
- 页面范围计算
- 标题规范化

**建议**：拆分为 `tree_builder.go`、`node_merger.go`、`title_utils.go` 等小文件。

#### 3. **循环依赖风险**

`MetaProcessor` 和 `TOCDetector` 之间存在紧密耦合：

```go
type MetaProcessor struct {
    llmClient   llm.LLMClient
    cfg         *config.Config
    tocDetector *TOCDetector  // 直接依赖
    docLanguage language.Language
}
```

**建议**：考虑引入接口解耦，或使用依赖注入框架。

---

## 三、核心功能审查

### 1. 索引生成流程

```
文档解析 → 页面分组 → TOC 检测 → 结构生成 → 节点合并 → 摘要生成
```

**亮点**：

- 支持三种处理模式：`ModeTOCWithPageNumbers`、`ModeTOCNoPageNumbers`、`ModeNoTOC`
- 动态限流器 `DynamicRateLimiter` 根据 API 反馈自适应调整并发
- 批量摘要生成使用 First Fit Decreasing 算法优化打包

**问题**：

- ~~`generator_toc.go:199` 中 `shouldSplit` 被硬编码为 `false`，大节点拆分功能被禁用：~~
  ```go
  shouldSplit := false  // 永远不会执行拆分逻辑
  ```

✅ **已完成（2026-04-01）**：已完全移除大节点拆分相关的所有死代码，包括两个处理函数和主流程调用代码，精简了约120行无用代码。

### 2. 文档解析

**优点**：

- PDF 解析支持文本层提取和 OCR 回退
- 智能检测混合内容 PDF（超过 50% 空白页触发 OCR）

**问题**：

- Markdown 解析将整个文档作为单页处理：

```go
// For Markdown, entire document is one page
page := Page{
    Number: 1,
    Text:   fullText.String(),
}
```

对于大型 Markdown 文件，这可能导致 token 超限。

### 3. 搜索功能

搜索模块过于简单，仅做 LLM 调用封装：

```go
func (s *Searcher) Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error) {
    // ...
    result, err := s.llmClient.Search(ctx, query, tree)
    // ...
}
```

**建议**：添加查询预处理、结果缓存、多轮对话支持等功能。

---

## 四、数据结构审查

### 1. Node 结构

```go
type Node struct {
    ID        string  `json:"id"`
    Title     string  `json:"title"`
    StartPage int     `json:"start_page"`
    EndPage   int     `json:"end_page"`
    Summary   string  `json:"summary,omitempty"`
    Children  []*Node `json:"children,omitempty"`
}
```

**问题**：

- 缺少 `Level` 字段记录层级深度
- 缺少 `Keywords` 字段存储关键词
- 缺少 `ContentHash` 用于增量更新检测

### 2. IndexTree 结构

```go
type IndexTree struct {
    Root         *Node            `json:"root"`
    TotalPages   int              `json:"total_pages"`
    DocumentInfo string           `json:"document_info"`
    GeneratedAt  time.Time        `json:"generated_at"`
    Version      int              `json:"version,omitempty"`
    LastModified time.Time        `json:"last_modified"`
    nodeMap      map[string]*Node `json:"-"`  // 非序列化
}
```

**优点**：

- `nodeMap` 提供 O(1) 节点查找
- 支持版本控制和增量更新

**问题**：

- `Merge` 方法在合并时创建 "Combined Document" 根节点，可能丢失原文档结构信息

### 3. TOCItem 结构

```go
type TOCItem struct {
    Structure     string `json:"structure"`
    Title         string `json:"title"`
    Page          *int   `json:"page,omitempty"`
    PhysicalIndex *int   `json:"physical_index,omitempty"`
    ListIndex     int    `json:"list_index,omitempty"`
    AppearStart   string `json:"appear_start,omitempty"`
    EndPage       int    `json:"-"`  // 临时字段
}
```

**问题**：

- ~~`EndPage` 是临时字段，但 `Page` 和 `PhysicalIndex` 是指针类型，语义不一致~~
  ```go
  ListIndex     int    `json:"list_index,omitempty"`
  EndPage       int    `json:"-"`  // 临时字段
  ```

✅ **已完成（2026-04-01）**：已将`ListIndex`和`EndPage`统一修改为指针类型，与其他字段保持一致，消除了类型不一致问题。
- `AppearStart` 使用字符串 `"yes"/"no"` 而非布尔值，增加类型不安全性

---

## 五、接口设计审查

### 1. LLMClient 接口

```go
type LLMClient interface {
    GenerateStructure(ctx context.Context, text string, lang language.Language) (*document.Node, error)
    GenerateSummary(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error)
    Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error)
    GenerateSimple(ctx context.Context, prompt string) (string, error)
    GenerateBatchSummaries(ctx context.Context, requests []*BatchSummaryRequest, lang language.Language) ([]*BatchSummaryResponse, error)
}
```

**问题**：

- 接口方法较多（5 个），可能违反接口隔离原则
- 缺少 `Embedding` 方法支持向量检索

### 2. MCP Tools 接口

MCP 服务提供三个工具：

- `generate_index` - 生成索引
- `search_index` - 搜索索引
- `update_index` - 更新索引

**优点**：

- 清晰的请求/响应类型定义
- 支持进度回调通知

**问题**：

- 缺少 `delete_index`、`list_indexes` 等管理工具
- 错误处理返回的是成功响应（`mcp.NewToolResultError` 返回 `(*mcp.CallToolResult, error)` 中 error 为 nil）

---

## 六、性能与安全审查

### 1. 性能优化亮点

- 使用 `sync.Pool` 复用 `strings.Builder`
- LRU 缓存减少重复 LLM 调用
- 动态限流器避免 API 过载
- 预计算字符串长度减少内存分配

### 2. 安全问题

#### 2.1 敏感信息处理

`config.go` 正确地将 API Key 与配置文件分离：

```go
// Only load sensitive credentials from environment variables/.env
if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
    cfg.OpenAIAPIKey = apiKey
}
```

#### 2.2 HTTP 认证

`http.go` 使用常量时间比较防止时序攻击：

```go
if subtle.ConstantTimeCompare([]byte(authHeader), []byte(expected)) == 1 {
    next.ServeHTTP(w, r)
    return
}
```

#### 2.3 潜在风险

- PDF 文件大小限制为 50MB，但无解压缩炸弹保护
- 缺少输入内容长度验证，可能导致内存耗尽

---

## 七、代码质量审查

### 1. 代码重复

`toc_detection.go` 和 `internal/utils/json.go` 中存在重复的 JSON 清理逻辑：

- `parseLLMJSONResponse`
- `trailingCommaRegex`

**建议**：统一使用 `internal/utils/json.go` 中的 `JSONCleaner`。

### 2. 错误处理

大部分错误处理得当，但存在一些问题：

```go
// 忽略错误的示例
defer file.Close() // nolint:errcheck // File cleanup in CLI command
```

虽然注释说明了原因，但建议使用更明确的错误处理模式。

### 3. 硬编码问题

- ~~`toc_extraction.go:84` 中 `maxAttempts = 5` 硬编码~~
- ~~`generator_toc.go:199` 中 `shouldSplit = false` 硬编码~~
- ~~语言检测样本大小 2000 字符硬编码~~

✅ **已完成（2026-04-01）**：已将所有硬编码配置值提取到Config结构体：
  - `LanguageDetectSampleSize` - 语言检测样本大小（默认2000）
  - `TOCExtractionMaxAttempts` - TOC提取最大重试次数（默认5）
  - `MaxTokensPerGroup` - 页面分组最大token数（默认20000）
  - 所有相关代码已修改为使用配置值而非硬编码

---

## 八、总结与建议

### 整体评价

这是一个架构清晰、功能完整的 RAG 系统实现，代码质量整体较高。主要优点包括：

- 良好的接口抽象和模块解耦
- 完善的错误处理和重试机制
- 性能优化考虑周全

### 优先改进建议

| 优先级 | 问题 | 状态 | 建议 |
|--------|------|------|------|
| 高 | indexer 目录文件过多 | ⏳ 待处理 | 按功能拆分子目录 |
| 高 | generator_simple.go 过长 | ⏳ 待处理 | 拆分为多个小文件 |
| 中 | 大节点拆分功能禁用 | ✅ **已完成** | 已完全移除相关死代码 |
| 中 | TOCItem 字段类型不一致 | ✅ **已完成** | 已统一为指针类型 |
| 低 | 硬编码配置值 | ✅ **已完成** | 已提取为配置项 |
| 低 | 缺少向量检索支持 | ⏳ 待处理 | 考虑添加 Embedding 接口 |
