# P0 Production Readiness Tasks - COMPLETE

All 11 P0 tasks completed and verified:

- [x] 恢复关键调试日志 (meta_processor_verify.go, toc_detection.go) - 14 debug logs
- [x] OCR 自动检测阈值从 0.5 降至 0.3 - pdf.go:81
- [x] 为 parseLLMJSONResponse 添加单元测试 (错误路径) - 8 tests, 92.9% coverage
- [x] 为 detectTOCPage 添加单元测试 (错误路径) - 2 tests, 100% coverage
- [x] 为 findTOCPages 添加单元测试 - 6 tests, 100% coverage
- [x] 为 extractTOCContent 添加单元测试 (错误路径) - 6 tests, 100% coverage
- [x] 为 parseTOCTransformerResponse 添加单元测试 (错误路径) - 2 tests, 100% coverage
- [x] 为 addPhysicalIndexToTOC 添加单元测试 - 6 tests, 100% coverage
- [x] 为 CheckTitleAppearance 添加单元测试 (错误路径) - 10 tests, 100% coverage
- [x] 为 CheckAllItemsAppearanceInStart 添加单元测试 (并发安全) - 10 tests + concurrent, 100% coverage
- [x] 运行测试验证所有 P0 修复 - PASS 11.705s, no races

## Verification

```bash
go test -race -count=1 ./pkg/indexer/...
# ok: github.com/xgsong/mypageindexgo/pkg/indexer (11.705s, no races)
```

## Commits

- eefcf1e - test: add error path tests for P0 coverage
- 76851c0 - chore: add .tasks tracking file for P0 completion
