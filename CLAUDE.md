# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 提供处理本代码仓库时的指导。

## 项目概述

PageIndex 是一个基于 LLM 的**无向量、基于推理的 RAG** 系统。核心特点：
- **无向量数据库**：使用文档结构和 LLM 推理进行检索，而非向量相似度搜索
- **无分块**：文档按自然语义章节组织，而非人工分块
- **类人检索**：模拟人类专家在复杂文档中导航和提取知识的方式
- **更好的可解释性**：基于推理的检索，具有完整的页面和章节引用可追溯性
- 在 FinanceBench 数据集上达到 **98.7% 准确率**，超越传统基于向量的 RAG

PageIndex 通过两个步骤执行检索：
1. 从文档生成层次化的"目录"树结构索引
2. 通过树搜索执行基于推理的检索

这是原始 Python 实现 ([VectifyAI/PageIndex](https://github.com/VectifyAI/PageIndex)) 的 Go 语言移植版本。

**详细技术规格请参见 [TECH_SPEC.md](./TECH_SPEC.md)。**

## 命令

### 依赖管理
```bash
go mod tidy              # 下载并整理依赖
go list -m all           # 列出所有依赖
go get <package>         # 添加新依赖
go get -u <package>      # 更新依赖
```

### 代码格式化
```bash
go fmt ./...             # 自动格式化所有 Go 文件
gofumpt -w ./...         # 增强格式化（可选）
go vet ./...             # 运行 go vet 检查问题
```

### 构建
```bash
go build -o pageindex ./cmd/pageindex
go build -race -o pageindex ./cmd/pageindex  # 带竞态检测器构建
```

### 运行
```bash
./pageindex --help
./pageindex generate --pdf /path/to/document.pdf    # 从 PDF 生成索引
./pageindex generate --md /path/to/document.md     # 从 Markdown 生成索引
./pageindex search --index index.json --query "your question"  # 搜索索引
```

### 支持的 CLI 命令

- `generate` - 从文档生成 PageIndex 树
  - `--pdf` - PDF 文件路径
  - `--md` - Markdown 文件路径
  - `--output` - 输出 JSON 文件路径（默认：output.json）
  - `--model` - 使用的 OpenAI 模型（默认：gpt-4o）
  - `--max-concurrency` - 最大并发 LLM 调用数

- `search` - 使用查询搜索生成的索引
  - `--index` - 生成的索引 JSON 路径
  - `--query` - 搜索查询
  - `--output` - 结果输出 JSON

### 测试
```bash
go test ./...
go test -v ./pkg/document  # 单包测试
go test -race ./...        # 带竞态检测器运行测试
```

### 清理
```bash
rm -f pageindex          # 仅删除二进制文件
```

## 依赖

所有依赖通过 Go 模块管理。需要 Go 1.25+。

核心依赖：
- `github.com/ledongthuc/pdf` - PDF 处理（纯 Go，无 CGO）
- `github.com/yuin/goldmark` - Markdown 处理
- `github.com/sashabaranov/go-openai` - OpenAI API 客户端
- `github.com/pkoukk/tiktoken-go` - Token 计数
- `github.com/spf13/viper` - 配置管理
- `github.com/urfave/cli/v2` - CLI 框架
- `golang.org/x/sync/errgroup` - 带错误传播的并发控制

## 配置

配置通过 Viper 管理：
- 支持 YAML、JSON 和环境变量
- 环境变量前缀为 `PAGEINDEX_`

必需配置：
```bash
export PAGEINDEX_OPENAI_API_KEY=your_openai_api_key_here
```

可选配置：
- `PAGEINDEX_OPENAI_BASE_URL` - 自定义 OpenAI 基础 URL（用于自托管模型）
- `PAGEINDEX_MAX_CONCURRENCY` - 最大并发 LLM 调用数（默认：5）
- `PAGEINDEX_MAX_TOKENS_PER_NODE` - 每组块最大 token 数（默认：16000）
- `PAGEINDEX_GENERATE_SUMMARIES` - 是否为所有节点生成摘要（true/false，默认：false）
- `PAGEINDEX_LOG_LEVEL` - 日志级别（debug、info、warn、error）

## 架构

### 项目结构

```
mypageindexgo/
├── cmd/
│   └── pageindex/
│       └── main.go         # CLI 入口点
├── pkg/
│   ├── config/             # 配置处理
│   │   └── config.go
│   ├── document/           # 文档处理核心
│   │   ├── parser.go       # 解析器接口
│   │   ├── pdf.go          # PDF 解析器
│   │   ├── markdown.go     # Markdown 解析器
│   │   └── tree.go         # 索引树数据结构
│   ├── llm/                # LLM 客户端抽象
│   │   ├── client.go       # LLM 接口
│   │   ├── openai.go       # OpenAI 实现
│   │   └── prompts.go      # 提示词模板
│   ├── tokenizer/          # Token 计数
│   │   └── tokenizer.go
│   ├── indexer/            # 索引生成和搜索
│   │   ├── generator.go    # 目录树生成
│   │   ├── processor.go    # 节点处理
│   │   └── search.go       # 基于推理的检索
│   └── output/             # 输出处理
│       └── json.go         # JSON 输出
├── internal/
│   └── utils/              # 私有工具
│       ├── json.go         # JSON 工具
│       └── errors.go       # 错误处理助手
└── test/
    └── fixtures/           # 测试文档
```

### 核心接口

1. **DocumentParser** - 文档解析抽象
   - 开箱即用支持 PDF 和 Markdown
   - 返回解析后的 `Document`，包含页面和元数据

2. **LLMClient** - LLM 客户端抽象
   - `GenerateStructure` - 从文本生成层次化节点结构
   - `GenerateSummary` - 为节点生成摘要
   - `Search` - 对索引树执行基于推理的搜索
   - 可扩展以支持 OpenAI 之外的其他 LLM 提供商

3. **IndexTree** - 完整的层次化文档索引
   - 根节点 `Node` 包含嵌套子节点
   - 每个节点具有标题、页面范围和可选摘要

### 并发模型

使用 `goroutine + errgroup.Group` 进行并发 LLM 调用：

```go
// 并发模式示例
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

- `errgroup.SetLimit()` 控制最大并发数以避免 API 速率限制
- 轻量级 goroutine（KB 栈）vs Python asyncio（MB 栈）
- 通过 errgroup 原生错误传播

### 设计原则

- **纯 Go 优先**：无 CGO，便于交叉编译（使用 pdfcpu 而非 go-fitz）
- **基于接口**：抽象允许交换实现（DocumentParser、LLMClient）
- **JSON 鲁棒性**：必须能处理 LLM 的非标准 JSON 输出，并进行适当的清理
- **显式错误处理**：Go 风格的 `(result, error)` 返回，使用 `%w` 包装
- **尽可能不可变**：核心数据结构在构造后不可变
- **并发安全**：Goroutine 安全设计，具有适当的同步机制

## 代码风格

遵循标准 Go 约定：
- **格式化**：提交前始终运行 `go fmt ./...`
- **注释**：所有公共标识符必须有适当的 Go 文档注释
- **包名**：简短、小写、单个单词（无下划线或驼峰命名）
- **错误处理**：显式 `(result, error)` 返回，使用 `fmt.Errorf("...: %w", err)` 包装
- **文件大小**：保持文件在 400 行以内，倾向于小而专注的包
- **不可变性**：核心数据结构在构造后应不可变

## 调试与性能分析

```bash
# CPU 和内存性能分析示例
go test -cpuprofile cpu.prof -memprofile mem.prof ./pkg/document
go tool pprof cpu.prof
```

设置 `PAGEINDEX_LOG_LEVEL` 环境变量以进行调试日志记录。
