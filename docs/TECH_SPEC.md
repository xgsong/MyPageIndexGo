# PageIndex Go 技术规格

## 项目概述

**PageIndex Go** 是一个基于LLM推理的无向量RAG（Retrieval-Augmented Generation）系统。

核心特点：
- 不需要向量数据库 / 文本分块 / embedding
- 通过LLM分析文档自动生成层次化目录树结构索引
- 基于目录树进行推理式检索，相比传统向量搜索准确率更高
- 在FinanceBench数据集上达到98.7%准确率

## 技术选型

| 功能模块 | 库选择 | 理由 |
|---------|--------|------|
| PDF文本提取 | [`github.com/ledongthuc/pdf`](https://github.com/ledongthuc/pdf) | 纯Go实现，无需CGO，专注于文本提取，API简洁 |
| PDF渲染（OCR用） | [`github.com/gen2brain/go-fitz`](https://github.com/gen2brain/go-fitz) | PDF渲染为图片，支持自定义DPI输出，OCR识别精度高 |
| OCR识别 | OpenAI兼容API (llama.cpp等) | 通过OpenAI兼容接口调用本地或云端OCR模型，支持批量处理 |
| Markdown处理 | [`github.com/yuin/goldmark`](https://github.com/yuin/goldmark) | 最流行的Go Markdown处理器，扩展性好 |
| OpenAI API | [`github.com/sashabaranov/go-openai`](https://github.com/sashabaranov/go-openai) | 社区标准实现，维护活跃，功能完整 |
| 令牌计数 | [`github.com/pkoukk/tiktoken-go`](https://github.com/pkoukk/tiktoken-go) | OpenAI tiktoken的Go移植，计数精度一致 |
| 配置管理 | [`github.com/spf13/viper`](https://github.com/spf13/viper) + [`github.com/joho/godotenv`](https://github.com/joho/godotenv) | 支持.env文件、环境变量、配置文件 |
| 结构化日志 | [`github.com/rs/zerolog`](https://github.com/rs/zerolog) | 高性能结构化日志库，支持JSON/文本输出，多日志级别 |
| 命令行 | [`github.com/urfave/cli/v2`](https://github.com/urfave/cli) | 简洁强大的Go命令行库 |
| 并发控制 | `golang.org/x/sync/errgroup` | 原生goroutine并发，更好的错误传播 |

## 项目结构

```
mypageindexgo/
├── cmd/pageindex/
│   └── main.go                 # CLI入口，支持 generate/search/update 命令
├── pkg/
│   ├── config/
│   │   └── config.go           # 配置加载与验证
│   ├── document/
│   │   ├── parser.go           # DocumentParser 适配器接口
│   │   ├── pdf.go              # PDF解析器实现
│   │   ├── markdown.go         # Markdown解析器实现
│   │   ├── tree.go             # Node/IndexTree 数据结构
│   │   ├── pdf_renderer.go      # PDF渲染为图片（OCR用）
│   │   └── llama_cpp_ocr_client.go  # OCR客户端接口
│   ├── llm/
│   │   ├── client.go           # LLMClient 接口定义
│   │   ├── openai.go           # OpenAI GPT实现
│   │   ├── prompts.go          # Prompt模板
│   │   ├── cached_client.go    # LLM响应缓存
│   │   └── lru_cache.go        # LRU缓存实现
│   ├── indexer/
│   │   ├── generator.go        # IndexGenerator 主入口
│   │   ├── generator_toc.go     # GenerateWithTOC 核心流程
│   │   ├── generator_structures.go  # 节点合并工具
│   │   ├── generator_summaries.go   # 摘要生成
│   │   ├── meta_processor.go   # MetaProcessor 模式选择
│   │   ├── meta_processor_*.go # 各处理模式实现
│   │   ├── toc_core.go        # TOCItem/TOCResult 数据结构
│   │   ├── toc_detection.go   # TOC检测与解析
│   │   ├── toc_extraction.go  # TOC提取逻辑
│   │   ├── toc_offset.go      # 页码偏移计算
│   │   ├── toc_verify_appearance.go  # 标题验证
│   │   ├── processor.go        # 节点处理接口
│   │   ├── search.go           # Searcher 检索实现
│   │   └── rate_limiter.go     # 动态并发控制
│   ├── tokenizer/
│   │   └── tokenizer.go        # 令牌计数
│   ├── language/
│   │   └── detect.go           # 文档语言检测
│   ├── logging/
│   │   └── logging.go         # zerolog配置
│   └── output/
│       └── json.go             # JSON序列化/反序列化
├── internal/utils/
│   ├── json.go                 # JSON解析与清理
│   └── retry.go                # 指数退避重试
├── test/
│   ├── fixtures/               # 测试文件
│   └── e2e/                    # 端到端测试
├── docs/                       # 文档
├── go.mod
├── go.sum
├── config.yaml                 # 配置文件
├── .env.example               # 环境变量示例
└── Makefile                    # 构建脚本
```

## 核心数据结构

### 文档解析

```go
// DocumentParser 是适配器接口，每个文件格式实现一个适配器
// 适配器将输入格式转换为统一的Document输出
type DocumentParser interface {
    Parse(r io.Reader) (*Document, error)
    SupportedExtensions() []string
    Name() string
}

// Document 表示解析后的统一文档
type Document struct {
    Pages    []Page              `json:"pages"`
    Metadata map[string]string   `json:"metadata,omitempty"`
    Language language.Language    `json:"language"` // 检测到的文档语言
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
    Root         *Node            `json:"root"`
    TotalPages   int              `json:"total_pages"`
    DocumentInfo string           `json:"document_info"`
    GeneratedAt  time.Time        `json:"generated_at"`
    Version      int              `json:"version,omitempty"`
    LastModified time.Time        `json:"last_modified"`
    nodeMap      map[string]*Node `json:"-"` // 内存索引，非序列化
}

// SearchResult 检索结果
type SearchResult struct {
    Query  string  `json:"query"`
    Answer string  `json:"answer"`
    Nodes  []*Node `json:"nodes"`
}
```

### TOC处理

```go
// TOCItem 表示单个目录条目
type TOCItem struct {
    Structure     string `json:"structure"`
    Title         string `json:"title"`
    Page          *int   `json:"page,omitempty"`
    PhysicalIndex *int   `json:"physical_index,omitempty"`
    ListIndex     int    `json:"list_index,omitempty"`
    AppearStart   string `json:"appear_start,omitempty"`
}

// TOCResult TOC检测结果
type TOCResult struct {
    TOCContent     string    `json:"toc_content"`
    TOCPageList    []int     `json:"toc_page_list"`
    PageIndexGiven bool      `json:"page_index_given"`
    Items          []TOCItem `json:"items"`
}

// ProcessingMode 处理模式
type ProcessingMode string
const (
    ModeTOCWithPageNumbers ProcessingMode = "process_toc_with_page_numbers"
    ModeTOCNoPageNumbers   ProcessingMode = "process_toc_no_page_numbers"
    ModeNoTOC             ProcessingMode = "process_no_toc"
)
```

### LLM客户端

```go
type LLMClient interface {
    GenerateStructure(ctx context.Context, text string, lang language.Language) (*document.Node, error)
    GenerateSummary(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error)
    Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error)
    GenerateSimple(ctx context.Context, prompt string) (string, error)
    GenerateBatchSummaries(ctx context.Context, requests []*BatchSummaryRequest, lang language.Language) ([]*BatchSummaryResponse, error)
}
```

## 处理流程

### GenerateWithTOC 主流程

```
输入: Document
  │
  ▼
[语言检测] ─── 检测文档语言（中文/英文等）
  │
  ▼
[TOC检测] ─── 检查前N页是否为目录
  │          - 按页检测是否为目录页
  │          - 提取目录内容
  │
  ▼
[模式选择]
  │
  ├─ ModeTOCWithPageNumbers: 有目录且有页码
  ├─ ModeTOCNoPageNumbers:   有目录但无页码
  └─ ModeNoTOC:              无目录
  │
  ▼
[MetaProcessor.Process] ─── 核心处理逻辑
  │
  ├─ processTOCWithPageNumbers:
  │   1. LLM提取TOC结构
  │   2. 采样内容页建立页码映射
  │   3. 计算逻辑页码到物理页码的偏移
  │   4. 修复缺失物理索引的条目
  │
  ├─ processTOCNoPageNumbers:
  │   1. LLM提取TOC结构
  │   2. 按组切分内容
  │   3. LLM为每个条目推断页码
  │
  └─ processNoTOC:
      1. 构建带页码标签的内容
      2. 按token限制切分
      3. LLM生成初始结构
      4. LLM增量扩展结构
  │
  ▼
[TOC验证] ─── 验证目录准确性
  │          - 计算准确率
  │          - 修复错误条目（可配置跳过）
  │
  ▼
[添加前言] ─── 如果首条目不是第1页，添加 Preface
  │
  ▼
[标题验证] ─── 验证首标题在内容起始处出现
  │
  ▼
[生成树结构] ─── TOCItem列表 → 树形结构
  │
  ▼
[大节点分裂] ─── 递归处理大节点
  │           - 超过MaxPagesPerNode且token超限
  │           - 使用MetaProcessor再次处理
  │
  ▼
[生成摘要] ─── （可选）批量生成节点摘要
  │
  ▼
输出: IndexTree
```

### Search 检索流程

```
输入: query + IndexTree
  │
  ▼
[LLM推理检索] ─── LLM理解查询，遍历树结构找到相关节点
  │
  ▼
[返回结果] ─── SearchResult { Query, Answer, Nodes }
```

## OCR功能技术设计

### 架构设计

OCR功能采用**运行时可选**设计：
- **运行时可选**：通过配置 `ocr_enabled` 控制是否启用OCR，无需重新编译
- **自动降级**：当PDF文本提取为空时，自动尝试使用OCR识别
- **多语言支持**：支持多种OCR提供商（ Llama.cpp、OpenAI OCR API等）

### 实现流程

```
PDF解析流程:
1. 尝试提取PDF内置文本层 → 非空则直接使用
2. 文本为空且OCR已启用 → 渲染PDF页面为图片 → OCR识别
3. 文本为空且OCR未启用 → 返回友好错误提示，引导用户启用OCR
```

## 性能优化架构

### 1. LLM调用缓存机制
- 基于文本哈希的响应缓存，使用`sync.Map`存储
- 支持配置TTL和搜索结果缓存

### 2. 指数退避重试机制
- 1s → 2s → 4s → 8s → 32s 指数退避
- 识别`Retry-After`头，区分可重试/不可重试错误

### 3. 节点ID哈希索引
- 预生成`nodeID → *Node`映射表
- O(1)查找，适合大型索引树

### 4. 动态并发控制
- 令牌桶算法，根据`X-RateLimit-*`头动态调整
- 最大化API配额利用率

### 5. 批量LLM调用
- 多个摘要请求合并为一个批量API调用
- 减少网络开销，API调用次数减少50%-70%

## 配置项

```yaml
# LLM配置
openai_base_url: "https://api.openai.com/v1"
openai_model: "gpt-4o"

# OCR配置
ocr_enabled: false
ocr_model: "GLM-OCR-Q8_0"
openai_ocr_base_url: "http://localhost:8080"
ocr_render_dpi: 150
ocr_concurrency: 5
ocr_timeout: 60

# 索引配置
max_concurrency: 20
max_pages_per_node: 10
max_tokens_per_node: 24000
generate_summaries: false
enable_batch_calls: true
batch_size: 20
toc_check_page_num: 20
max_token_num_each_node: 20000
skip_toc_fix: false
skip_appearance_check: false

# 缓存配置
enable_llm_cache: true
llm_cache_ttl: 3600
enable_search_cache: false

# 日志配置
log_level: "info"
```

## 并发模型

Python版本使用 `asyncio + ThreadPoolExecutor`。Go版本使用 `goroutine + errgroup.Group`：

```go
group, ctx := errgroup.WithContext(ctx)
group.SetLimit(cfg.MaxConcurrency)

for _, pageGroup := range pageGroups {
    pageGroup := pageGroup
    group.Go(func() error {
        node, err := llmClient.GenerateStructure(ctx, pageGroup.Text)
        if err != nil {
            return fmt.Errorf("failed to generate structure: %w", err)
        }
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

## 设计模式

### 适配器模式
支持多文件格式：
- 每个格式实现一个`DocumentParser`适配器
- 所有适配器输出统一的`Document`结构
- 下游索引流程无需修改即可支持新格式

### 策略模式
TOC处理支持多种模式：
- `ModeTOCWithPageNumbers`: 有目录有页码
- `ModeTOCNoPageNumbers`: 有目录无页码
- `ModeNoTOC`: 无目录

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

1. **纯Go优先**：使用纯Go实现的文本提取库，避免CGO简化交叉编译

2. **JSON鲁棒性**：Python版本有复杂的JSON修复逻辑，已在 `internal/utils/json.go` 实现JSON清理，支持15种错误恢复模式

3. **限流控制**：通过errgroup.SetLimit和动态RateLimiter合理控制并发

4. **令牌精度验证**：tiktoken-go和Python tiktoken计数结果一致

5. **环境配置**：配置优先从 `config.yaml` 读取，敏感信息从 `.env` 或环境变量读取

6. **多格式扩展**：新增格式只需要实现 `DocumentParser` 接口，并在 `ParserRegistry` 注册
