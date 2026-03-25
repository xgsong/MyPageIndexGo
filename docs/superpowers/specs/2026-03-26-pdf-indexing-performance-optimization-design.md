# PDF索引生成性能优化方案

## 一、问题分析

### 1.1 当前耗时分布（基于日志）

```
总耗时：321.5秒（21页，42节点）

├── Document parsed successfully        ~1秒
├── Detected document language          ~1秒
├── TOC检测 (逐页LLM调用)
│   ├── Page has TOC page=4            ~12秒 (每页检测一次)
│   ├── Page has TOC page=14          ~33秒
│   └── 多次detectTOCPage调用           ~25秒
├── Meta processor
│   ├── Content grouped groups=1       ~65秒
│   ├── TOC verification               ~65秒 (accuracy=0.80, 33正确, 8错误)
│   └── fixIncorrectTOC (重试3次)       未计入
├── Checking title appearance          ~66秒 (41 items)
└── Tree structure created             ~0.1秒
```

### 1.2 瓶颈识别

| 瓶颈 | 当前状态 | 优化潜力 |
|------|----------|----------|
| TOC批量检测 | 逐页调用LLM，21页=21次调用 | 合并为2-3次批量调用 |
| TOC验证流程 | verifyTOC + fixIncorrectTOC多轮 | 简化为只验证不修复 |
| title appearance检查 | 41 items逐个并发检查 | 可跳过或简化 |
| 并发限制 | 动态限制初始值10 | 提高到15-20 |
| 分组策略 | 按token动态分组 | 固定页数分组减少组数 |

## 二、优化方案（方案A：激进优化）

### 2.1 批量TOC检测

**问题**：当前`findTOCPages`逐页调用`detectTOCPage`，每页一次LLM调用。

**优化**：将前N页内容合并为一批，一次性检测是否包含TOC。

```go
// 优化前：21次LLM调用
for i := startIndex; i < len(pages); i++ {
    isTOC, _ := d.detectTOCPage(ctx, pages[i])
}

// 优化后：2-3次LLM调用
// 将前5页合并为一批进行检测
batches := [][]string{
    pages[0:5],   // 批量1
    pages[5:10],  // 批量2
    // ...
}
```

**预期节省**：~40秒

### 2.2 简化验证流程

**问题**：`verifyTOC`检查每个item后，`fixIncorrectTOC`对8个错误项进行3次重试修复。

**优化**：
1. 验证后只记录错误项，不进行修复
2. 或者完全跳过verify步骤（接受~80%准确率）

**预期节省**：~30-65秒

### 2.3 提高并发上限

**问题**：当前`MaxConcurrency=10`，`DynamicRateLimiter`初始并发为10。

**优化**：
- 将`MaxConcurrency`默认值从10提高到20
- 调整`DynamicRateLimiter`的`maxConcurrency`上限

**预期节省**：~30-50秒（取决于API处理能力）

### 2.4 优化分组策略

**问题**：当前`pageListToGroupText`按token动态分组，可能产生较多小组。

**优化**：采用固定页数分组策略，减少分组数量。

```go
// 优化前：按token动态分组，可能3-4组
// 优化后：固定每5页一组，减少为1-2组
```

**预期节省**：~10-20秒

### 2.5 跳过或简化appearance检查

**问题**：`CheckAllItemsAppearanceInStart`对41个item逐个检查。

**优化选项**：
- 选项A：完全跳过（节省66秒，但丢失appear_start信息）
- 选项B：只检查前10个item（节省50秒）
- 选项C：降低检查频率（每5个item检查1个）

**预期节省**：选项A约66秒，选项B约50秒

## 三、预期效果

### 优化后的耗时估算

| 阶段 | 当前耗时 | 优化后耗时 | 节省 |
|------|----------|------------|------|
| TOC检测（批量） | ~70秒 | ~20秒 | ~50秒 |
| 分组+初始生成 | ~65秒 | ~40秒 | ~25秒 |
| 验证（简化） | ~65秒 | ~20秒 | ~45秒 |
| appearance检查 | ~66秒 | ~10秒 | ~56秒 |
| 其他开销 | ~55秒 | ~10秒 | ~45秒 |
| **总计** | **321秒** | **~100秒** | **~221秒** |

### 风险评估

| 优化项 | 风险等级 | 说明 |
|--------|----------|------|
| 批量TOC检测 | 低 | 检测逻辑不变，只是批量提交 |
| 简化验证 | 中 | 可能降低准确率至75-80% |
| 提高并发 | 低 | 本地模型无API限制 |
| 跳过appearance | 高 | 可能影响树结构的准确性 |

## 四、配置变更

```yaml
# config.yaml 新增配置项
indexer:
  max_concurrency: 20          # 从10提高到20
  enable_batch_toc_detect: true # 启用批量TOC检测
  skip_appearance_check: false # 是否跳过appearance检查
  verify_toc: true            # 是否进行TOC验证
  fix_incorrect_toc: false    # 是否修复错误的TOC项
```

## 五、实施步骤

1. **第一阶段：批量TOC检测**
   - 修改`toc_detection.go`中的`findTOCPages`方法
   - 实现批量检测逻辑

2. **第二阶段：并发优化**
   - 修改`config.go`默认`MaxConcurrency`
   - 调整`rate_limiter.go`参数

3. **第三阶段：验证流程简化**
   - 修改`meta_processor_verify.go`
   - 添加配置开关

4. **第四阶段：appearance检查优化**
   - 根据准确性要求决定优化程度

## 六、验证方法

```bash
# 运行优化后的代码
./pageindex generate --pdf ./test/fixtures/test.pdf --output index.json

# 对比优化前后：
# - 总耗时是否从321秒降到100秒左右
# - 生成结果的节点数量是否保持42个
# - 树结构是否合理
```
