# 代码审查报告
审查日期: 2026-03-27

## ✅ 项目优点
- 遵循标准 Go 项目布局，架构清晰，依赖关系合理，无循环依赖
- `go vet` 检查全部通过，代码语法质量良好
- 良好的接口抽象设计（如 LLMClient、OCRClient），扩展性强
- 完善的错误处理和降级 fallback 机制
- 完整的测试体系，包含单元测试、集成测试和端到端测试
- 各包职责明确，符合单一职责原则

---

## ❌ 违反架构规范问题

### 1. 文件行数超标（Go 文件限制 ≤250 行）
共发现 10 个文件超过限制：
| 文件路径 | 行数 | 严重程度 |
|---------|------|----------|
| pkg/indexer/generator_test.go | 861 | 高 |
| cmd/pageindex/main.go | 455 | 高 |
| pkg/llm/openai.go | 420 | 高 |
| pkg/indexer/indexer_test.go | 367 | 高 |
| pkg/document/tree_test.go | 364 | 高 |
| pkg/indexer/toc_detection.go | 361 | 高 |
| pkg/indexer/meta_processor.go | 320 | 中 |
| pkg/indexer/meta_processor_grouping.go | 297 | 中 |
| pkg/config/config.go | 255 | 低 |

### 2. 文件夹文件数超标（限制 ≤8 个文件/文件夹）
| 文件夹路径 | 文件数 | 严重程度 |
|-----------|--------|----------|
| pkg/indexer/ | 26 | 高 |
| pkg/llm/ | 14 | 高 |
| pkg/document/ | 13 | 高 |

---

## ❌ 代码质量问题
- **未使用代码**：在 pkg/indexer/toc_detection.go 中发现 3 个未使用函数：
  1. `batchTOCDetectorPrompt()`
  2. `(*TOCDetector).detectTOCPagesBatch()`
  3. `(*TOCDetector).detectPageIndex()`

---

## 📋 优化建议

### 1. 文件夹结构重构建议
```
pkg/indexer/ → 拆分为：
├── toc/          # TOC 检测、提取、验证相关代码
├── generator/    # 索引生成相关代码
├── meta/         # 元数据处理相关代码
└── search/       # 搜索功能相关代码

pkg/llm/ → 拆分为：
├── clients/      # OpenAI、缓存客户端实现
├── ocr/          # OCR 相关实现
└── cache/        # LRU 缓存实现

pkg/document/ → 拆分为：
├── parser/       # PDF、Markdown 解析
├── ocr/          # OCR 客户端实现
└── renderer/     # PDF 渲染相关
```

### 2. 大文件拆分建议
- 将 `main.go` 按 CLI 命令拆分为 `generate.go`、`search.go`、`update.go` 等单独文件
- 大型测试文件按测试场景拆分为多个小文件，提高可读性
- 对于业务逻辑文件，按功能模块拆分到不同文件

### 3. 代码质量优化
- 清理或实现上述未使用的 TOC 检测函数
- 配置 golangci-lint 到 CI 流程中，自动捕获未使用代码等问题
- 建立代码提交前的 lint 检查机制，从流程上保障代码规范
