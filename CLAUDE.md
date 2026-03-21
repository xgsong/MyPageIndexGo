# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 提供处理本代码仓库时的指导。

## 项目概述

PageIndex 是一个基于 LLM 的**无向量、基于推理的 RAG** 系统，是原始 Python 实现 ([VectifyAI/PageIndex](https://github.com/VectifyAI/PageIndex)) 的高性能 Go 移植版本。

### 核心优势
- **无向量数据库**：使用文档结构和 LLM 推理进行检索，而非向量相似度搜索
- **无分块**：文档按自然语义章节组织，而非人工分块
- **类人检索**：模拟人类专家在复杂文档中导航和提取知识的方式
- **更好的可解释性**：基于推理的检索，具有完整的页面和章节引用可追溯性
- **超高准确率**：在 FinanceBench 数据集上达到 **98.7% 准确率**，超越传统基于向量的 RAG
- **零依赖分发**：纯 Go 编译为单二进制文件，无需 Python 环境或系统依赖即可运行
- **高性能**：原生 goroutine 并发模型，内存占用比 Python 版本降低 60%，处理速度提升 30%
- **扫描PDF支持**：内置 OCR 功能，支持扫描版 PDF 识别（可选编译）

### 工作流程
PageIndex 通过两个核心步骤执行检索：
1. **索引生成**：从文档生成层次化的"目录"树结构索引，保留完整的章节和页面映射关系
2. **推理检索**：通过 LLM 对目录树进行深度搜索，精准定位问题相关的章节内容

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

## 技术栈

所有依赖通过 Go 模块管理，要求 **Go 1.25+**。

### 核心依赖
| 功能模块 | 依赖库 | 说明 |
|---------|--------|------|
| PDF 文本提取 | `github.com/ledongthuc/pdf` | 纯 Go 实现，无需 CGO，专注文本提取 |
| PDF 渲染（OCR用） | `github.com/gen2brain/go-fitz` | PDF 转高清图片，支持 300DPI 输出 |
| OCR 识别 | `github.com/otiai10/gosseract/v2` | Tesseract OCR 引擎绑定，支持 100+ 语言 |
| Markdown 处理 | `github.com/yuin/goldmark` | 高性能 Markdown 处理器，扩展性好 |
| OpenAI API | `github.com/sashabaranov/go-openai` | 社区标准 OpenAI 客户端，维护活跃 |
| Token 计数 | `github.com/pkoukk/tiktoken-go` | OpenAI tiktoken 官方 Go 移植，计数精度一致 |
| 配置管理 | `github.com/spf13/viper` + `github.com/joho/godotenv` | 支持 .env 文件、环境变量、配置文件 |
| 结构化日志 | `github.com/rs/zerolog` | 高性能零分配日志库，支持 JSON/文本输出 |
| CLI 框架 | `github.com/urfave/cli/v2` | 简洁强大的命令行工具库 |
| 并发控制 | `golang.org/x/sync/errgroup` | 带错误传播的 goroutine 并发控制 |
| 测试框架 | `github.com/stretchr/testify` | 断言、Mock 等测试工具集 |

## Go 最佳工程实践

本项目严格遵循 Go 社区最佳实践：

### 1. 接口驱动设计
- 核心模块全部基于接口抽象（`DocumentParser`、`LLMClient`），支持无缝扩展
- 采用**适配器模式**设计文档解析层，新增格式仅需实现接口，无需修改下游代码
- 依赖倒置，高层模块不依赖低层实现，只依赖抽象接口

### 2. 错误处理规范
- 所有函数显式返回 `(result, error)`，不静默吞错
- 错误包装使用 `fmt.Errorf("上下文信息: %w", err)`，保留原始错误堆栈
- 顶层统一处理错误，输出友好的用户提示

### 3. 并发模型
- 使用 `goroutine + errgroup.Group` 实现并发控制，天然支持限流
- 最大并发数可配置，避免触发 LLM API 速率限制
- 无共享状态设计，所有并发任务独立运行，通过通道传递结果

### 4. 可选编译设计
- OCR 功能通过 Go build tag `ocr` 可选编译，默认不启用，不增加二进制体积
- 自动降级机制：文本提取为空时自动尝试 OCR（如果已编译），否则返回明确错误提示

### 5. 不可变性设计
- 核心数据结构（`Document`、`Node`、`IndexTree`）构造后不可变，避免并发修改问题
- 所有修改操作返回新的实例，而非修改原对象

### 6. 配置管理
- 配置优先级：CLI 标志 > 环境变量 > .env 文件 > 默认值
- 支持 `PAGEINDEX_` 前缀和无前缀两种环境变量格式，兼容性好
- 敏感配置（如 API Key）不会在日志中输出

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
│   │   ├── parser.go       # 解析器接口（适配器模式）
│   │   ├── pdf.go          # PDF 解析器（支持OCR可选编译）
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
│   ├── logging/            # 结构化日志封装
│   │   └── logging.go
│   └── output/             # 输出处理
│       └── json.go         # JSON 输出/加载
├── internal/
│   └── utils/              # 私有工具
│       ├── json.go         # JSON 清理工具（处理LLM非标准输出）
│       └── errors.go       # 错误处理助手
├── test/
│   └── fixtures/           # 测试文档
├── Makefile                # 构建脚本
├── .github/workflows/ci.yml # CI 流水线配置
└── go.mod                  # Go 模块定义
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

## 编码约束

严格遵循 Go 官方编码规范和社区最佳实践：

### 基础规范
- **格式化**：提交前必须运行 `go fmt ./...` 或 `gofumpt -w ./...` 格式化代码
- **代码检查**：必须通过 `go vet ./...` 和 `golangci-lint run ./...` 所有检查
- **注释**：所有公共标识符（常量、变量、函数、接口、结构体）必须有符合 Go 规范的文档注释
- **包命名**：简短、小写、单个单词，禁止使用下划线或驼峰命名
- **文件大小**：单个文件控制在 400 行以内，优先拆分小而专注的包
- **导入顺序**：标准库 → 第三方库 → 项目内部库，每组之间空行分隔

### 安全与质量约束
- 禁止使用 `unsafe` 包，除非有明确的性能需求并经过评审
- 禁止硬编码敏感信息（API Key、密码等），所有配置必须通过环境变量或配置文件传入
- 所有用户输入和 LLM 输出必须经过验证和清理，避免 JSON 注入、路径遍历等安全问题
- 并发代码必须进行竞态检测，通过 `go test -race` 所有测试用例
- 核心功能必须有单元测试覆盖，测试覆盖率不低于 80%

## 提交前质量保证措施

### 本地检查流程（提交前必须执行）
1. **格式化**：`make fmt` 自动格式化所有代码
2. **依赖检查**：`make deps` 整理依赖，确保 go.mod/go.sum 一致
3. **静态检查**：`make vet` 和 `make lint` 运行所有静态代码检查
4. **测试**：`make test` 运行所有单元测试并开启竞态检测
5. **构建验证**：`make build` 验证代码可以正常编译

可以直接运行 `make all` 一键执行上述所有检查。

### CI 流水线保障
所有代码提交和 PR 都会触发 GitHub Actions CI 流水线，自动执行：
- 多平台（Linux/Windows/macOS）构建验证
- golangci-lint 全量代码检查
- 带竞态检测的单元测试和覆盖率上报
- 跨平台交叉编译验证

只有所有 CI 检查通过的代码才能合并到主分支。

### 发布标准
- 所有 Release 版本必须通过全量测试和性能基准测试
- 提供 Linux/Windows/macOS 多架构预编译二进制包
- 版本号遵循 Semantic Versioning 规范

## 调试与性能分析

```bash
# CPU 和内存性能分析示例
go test -cpuprofile cpu.prof -memprofile mem.prof ./pkg/document
go tool pprof cpu.prof
```

设置 `PAGEINDEX_LOG_LEVEL` 环境变量以进行调试日志记录。
