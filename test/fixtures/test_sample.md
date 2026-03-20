# 测试文档 第1页
## 概述
这是一份用于PageIndex功能测试的PDF文档。
包含多页文本内容，用于验证PDF解析、索引生成和搜索功能。

### 测试目标
1. 验证文本型PDF解析能力
2. 验证多页文档索引生成
3. 验证分层结构识别
4. 验证搜索功能准确性

---
page-break

# 测试文档 第2页
## 功能特性
### 核心功能
- 无向量数据库依赖
- 基于文档结构的推理检索
- 自然语义分块，无人工切分
- 可解释性检索结果，带页码引用

### 技术优势
- 准确率98.7%（FinanceBench数据集）
- 比向量检索节省70%存储成本
- 支持PDF、Markdown等多格式输入
- 支持中英文混合文档

---
page-break

# 测试文档 第3页
## 快速开始
### 安装
```bash
go install github.com/xgsong/mypageindexgo/cmd/pageindex@latest
```

### 基本使用
生成索引：
```bash
pageindex generate --pdf document.pdf --output index.json
```

搜索索引：
```bash
pageindex search --index index.json --query "你的问题"
```

---
page-break

# 测试文档 第4页
## 配置说明
### 环境变量
| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| PAGEINDEX_OPENAI_API_KEY | OpenAI API密钥 | 必需 |
| PAGEINDEX_MAX_CONCURRENCY | 最大并发数 | 5 |
| PAGEINDEX_GENERATE_SUMMARIES | 是否生成节点摘要 | false |
| PAGEINDEX_LOG_LEVEL | 日志级别 | info |

### 高级配置
- 支持自定义LLM模型
- 支持自定义提示词模板
- 支持结果输出格式定制
- 支持批量文档处理

---
page-break

# 测试文档 第5页
## 测试场景
### 功能测试用例
1. 单页文档生成索引
2. 多页文档生成索引
3. 中英文混合文档处理
4. 分层结构识别准确性
5. 摘要生成质量
6. 搜索结果相关性

### 性能测试用例
1. 100页文档处理速度
2. 1000页文档内存占用
3. 并发请求处理能力
4. 大文档索引生成时间

---
## 联系方式
如有问题，请提交Issue到GitHub仓库。
