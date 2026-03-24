# Code Review Findings

> Reviewed: 2026-03-24
> Scope: All 47 production source files (~8,800 lines)
> Reviewer: Claude Opus 4.6

---

## CRITICAL

### CR-001 Data Race — `completedBatches` not thread-safe

- **File:** `pkg/indexer/generator_summaries.go:153-211`
- **Description:** `completedBatches` is a plain `int` incremented inside concurrent goroutines launched by `errgroup`, without any synchronization. This is a data race that will cause undefined behavior under `-race` flag.
- **Code:**
  ```go
  completedBatches := 0  // line 153
  for _, batch := range batches {
      eg.Go(func() error {
          // ...
          completedBatches++  // line 206 — DATA RACE
      })
  }
  ```
- **Fix:** Use `atomic.Int32` like other counters in the codebase (e.g., `generator_structures.go:22`).

---

### CR-002 Goroutine loop variable capture (Go <1.22 risk)

- **File:** `pkg/indexer/generator_summaries.go:155-215`
- **Description:** The `batch` variable in `for _, batch := range batches` is captured by the closure without rebinding (`batch := batch`). In Go versions before 1.22, all goroutines would share the **last** value of `batch`.
- **Also at:** `pkg/indexer/toc_verify_appearance.go:143-157` — `task` variable in goroutine.
- **Fix:** Add `batch := batch` before the `eg.Go` call, or ensure `go.mod` specifies Go 1.22+.

---

### CR-003 LRU Cache double-locking — dangling reference

- **File:** `pkg/llm/cached_client.go:44-71`
- **Description:** The `Get` method uses a problematic double-lock pattern. Between `RUnlock` and the second `Lock`, another goroutine could evict the entry, making `entry.element` a dangling reference. The TTL check at line 53 also accesses `entry.timestamp` without a lock after releasing `RLock`.
- **Code:**
  ```go
  func (c *LRUCache) Get(key string) (any, bool) {
      c.mu.RLock()
      entry, exists := c.entries[key]
      c.mu.RUnlock()       // Released here

      // entry.timestamp accessed WITHOUT lock
      if c.ttl > 0 && time.Since(entry.timestamp) > c.ttl { ... }

      c.mu.Lock()           // Re-acquired — entry may be evicted by now
      c.lruList.MoveToFront(entry.element)  // Dangling reference?
      c.mu.Unlock()
  }
  ```
- **Fix:** Hold a single write lock for the entire Get operation, or restructure to avoid accessing entry fields between lock releases.

---

## HIGH

### CR-004 File size violation — `cmd/pageindex/main.go` (455 lines)

- **File:** `cmd/pageindex/main.go`
- **Guideline:** Static language files ≤ 250 lines
- **Description:** Contains 3 nearly identical action functions (`generateAction`, `searchAction`, `updateAction`) with duplicated:
  - Config loading + CLI flag override
  - Logging setup
  - LLM client creation + cache wrapping
  - OCR client creation
  - Document parser selection
- **Fix:** Extract shared helpers: `loadConfigWithCLI(c)`, `createLLMClient(cfg)`, `parseDocument(cfg, c)`.

---

### CR-005 File size violation — `pkg/llm/openai.go` (442 lines)

- **File:** `pkg/llm/openai.go`
- **Guideline:** Static language files ≤ 250 lines
- **Description:** `GenerateBatchSummaries` (lines 306-421) contains **3 identical fallback loops** that individually call `GenerateSummary`. Each loop is ~15 lines of duplicate code.
- **Fix:** Extract `fallbackToIndividualSummaries(ctx, requests, lang)` helper and call it from all 3 places. Additionally, split the file into `openai_core.go` (client + createChatCompletion) and `openai_methods.go` (individual API methods).

---

### CR-006 Folder structure violation — `pkg/indexer/` has 22 source files

- **File:** `pkg/indexer/`
- **Guideline:** Folders should have ≤ 8 files
- **Description:** The `pkg/indexer/` package is a "God package" with 22 non-test Go files. It mixes TOC detection, meta processing, tree generation, page grouping, rate limiting, and search into a single package.
- **Suggested split:**
  - `pkg/indexer/toc/` — TOC detection, extraction, verification, appearance (~7 files)
  - `pkg/indexer/meta/` — MetaProcessor and all its helpers (~6 files)
  - `pkg/indexer/` — Generator, processor, search, rate limiter (keep ~8 files)

---

### CR-007 Duplicate function — `buildContentWithTags`

- **Files:**
  - `pkg/indexer/toc_detection.go:104-111` (package-level function)
  - `pkg/indexer/meta_processor_helpers.go:115-122` (MetaProcessor method, calls standalone)
- **Description:** Two identical functions with the same name exist. The method version just delegates to the standalone one.
- **Fix:** Remove one of them and use the survivor consistently.

---

### CR-008 File size violation — `pkg/llm/cached_client.go` (268 lines)

- **File:** `pkg/llm/cached_client.go`
- **Guideline:** Static language files ≤ 250 lines
- **Description:** LRU cache implementation (122 lines) and cached client wrapper (147 lines) are combined in one file.
- **Fix:** Extract `LRUCache` to `pkg/llm/lru_cache.go`.

---

### CR-009 File size violation — `pkg/indexer/meta_processor_grouping.go` (275 lines)

- **File:** `pkg/indexer/meta_processor_grouping.go`
- **Guideline:** Static language files ≤ 250 lines
- **Fix:** Extract `processNonePageNumbers` (lines 189-275) to its own file.

---

### CR-010 Dead code — `toc_verifier.go` (236 lines)

- **File:** `pkg/indexer/toc_verifier.go`
- **Description:** The `TOCVerifier` type is created in `MetaProcessor` (`meta_processor.go:39`) but its methods are **never called** in any production code path. After the algorithm alignment, verification was rewritten to use `AppearanceChecker`-based approach in `meta_processor_verify.go`.
- **Dead types/methods:**
  - `VerificationResult`
  - `TOCVerifier.verifyTOCEntry`
  - `TOCVerifier.VerifyTOC`
  - `TOCVerifier.fixIncorrectTOCEntry`
  - `TOCVerifier.FixIncorrectTOC`
  - `TOCVerifier.FixIncorrectTOCWithRetries`
- **Fix:** Remove `toc_verifier.go` entirely. Remove the `tocVerifier` field from `MetaProcessor`.

---

### CR-011 Dead code — `processor_merge.go` simple page-split functions

- **File:** `pkg/indexer/processor_merge.go:45-142`
- **Description:** `ProcessLargeNodeRecursively` and `ProcessLargeNodesInTree` use a simple page-split approach (no LLM). After the algorithm alignment, `processLargeNodesWithMetaProcessor` in `generator_toc.go` replaced these with LLM-based splitting. These functions are never called in production code.
- **Fix:** Remove `ProcessLargeNodeRecursively` and `ProcessLargeNodesInTree`. Keep only `MergeNodes` in the file.

---

### CR-012 Dead code — `MetaProcessor.ProcessLargeNodeRecursively` in `meta_processor_toc_gen.go`

- **File:** `pkg/indexer/meta_processor_toc_gen.go:142-190`
- **Description:** This `TOCItemWithNodes`-based version is never called. The actual large node processing uses `generator_toc.go:processLargeNodesWithMetaProcessor` which operates on `document.Node` directly.
- **Also dead:** `TOCItemWithNodes` type in `meta_processor_merge.go:11-14`.
- **Fix:** Remove the method and the unused type.

---

## MEDIUM

### CR-013 Unbounded goroutines in `CheckAllItemsAppearanceInStart`

- **File:** `pkg/indexer/toc_verify_appearance.go:139-160`
- **Description:** Launches one goroutine per TOC item with no concurrency limit. For large documents with hundreds of TOC items, this could overwhelm the LLM API with simultaneous requests.
- **Fix:** Use `errgroup` with `SetLimit()` or a semaphore channel.

---

### CR-014 OCR `extractWithOCR` is a permanent stub

- **File:** `pkg/document/pdf.go:170-181`
- **Description:** This method always returns an error: `"OCR feature requires PDF rendering support..."`. It was never implemented, yet the config and CLI expose OCR as a usable feature (`--ocr-enabled`). This misleads users.
- **Fix:** Either implement OCR or remove the feature flag and related code until it's ready.

---

### CR-015 `parseLLMJSONResponse` replaces ALL "None" with "null"

- **File:** `pkg/indexer/toc_detection.go:84`
- **Code:**
  ```go
  content = strings.ReplaceAll(content, "None", "null")
  ```
- **Description:** This replaces "None" everywhere in the string, including inside string values like titles. Example: `"title": "None of the above"` becomes `"title": "null of the above"`.
- **Fix:** Use a targeted regex that only replaces standalone `None` at JSON value positions, e.g.:
  ```go
  content = regexp.MustCompile(`:\s*None\b`).ReplaceAllString(content, ": null")
  content = regexp.MustCompile(`\[\s*None\b`).ReplaceAllString(content, "[ null")
  content = regexp.MustCompile(`,\s*None\b`).ReplaceAllString(content, ", null")
  ```

---

### CR-016 `fmt.Printf` debug output in production code

- **File:** `pkg/llm/openai.go:178-179`
- **Code:**
  ```go
  fmt.Printf("DEBUG: Raw LLM response (first %d chars): %q\n", showLen, content[:showLen])
  fmt.Printf("DEBUG: Total response length: %d\n", len(content))
  ```
- **Description:** Debug output goes to stdout, mixing with user-visible output. Should use structured logging.
- **Fix:** Replace with `log.Debug().Str("content", content[:showLen]).Int("length", len(content)).Msg("Raw LLM response")`.

---

### CR-017 Inconsistent page indexing (0-based vs 1-based)

- **Files:** Multiple files across `pkg/indexer/`
- **Description:** No consistent convention for page indices:
  | Location | Convention |
  |----------|-----------|
  | `findTOCPages` | 0-based (iterates `pages` slice) |
  | `CheckTitleAppearance` | `pageIdx := *item.PhysicalIndex - startIndex` |
  | `CheckAllItemsAppearanceInStart` | `pageIdx := *item.PhysicalIndex - 1` (assumes 1-based) |
  | `samplePages` | `for i := startIndex - 1; i < endIndex` (mixed) |
  | `verifyTOCEntry` | `pageContent := pages[pageIdx-1]` (1-based) |
- **Risk:** Off-by-one bugs, especially when `startIndex != 1`.
- **Fix:** Document and enforce a single convention. Recommend: `PhysicalIndex` is always 1-based (matching PDF convention). All slice access should use `pageTexts[physicalIndex - 1]`.

---

### CR-018 Missing context cancellation in concurrent appearance check

- **File:** `pkg/indexer/toc_verify_appearance.go:139-160`
- **Description:** Goroutines use `sync.WaitGroup` but don't check for context cancellation. If the parent context is cancelled, all goroutines continue running until their LLM calls eventually time out.
- **Fix:** Use `errgroup.WithContext(ctx)` which automatically cancels remaining goroutines on first error or context cancellation.

---

### CR-019 Error swallowing in `convertPhysicalIndexToInt`

- **Files:**
  - `pkg/indexer/meta_processor_toc_gen.go:63-64`
  - `pkg/indexer/meta_processor_toc_gen.go:134`
- **Code:**
  ```go
  idx, _ := convertPhysicalIndexToInt(physicalIndexStr)
  items[i].PhysicalIndex = &idx  // idx = 0 on error!
  ```
- **Description:** If conversion fails, `idx` is 0 and it silently sets `PhysicalIndex = &0`. This could assign page 0 (invalid) to TOC items.
- **Fix:** Check the error and skip setting `PhysicalIndex` on failure:
  ```go
  if idx, err := convertPhysicalIndexToInt(physicalIndexStr); err == nil {
      items[i].PhysicalIndex = &idx
  }
  ```

---

### CR-020 Mutation of input parameters / shared pointer

- **File:** `pkg/indexer/generator_simple.go:19-23`
- **Code:**
  ```go
  for i := range items {
      if items[i].PhysicalIndex == nil && items[i].Page != nil {
          items[i].PhysicalIndex = items[i].Page  // shares pointer!
      }
  }
  ```
- **Description:** This makes `PhysicalIndex` point to the **same memory** as `Page`. Any subsequent modification to one silently affects the other. This violates the immutability guideline.
- **Other mutation sites:**
  - `meta_processor_verify.go:104-109` — modifies items in-place
  - `toc_verify_appearance.go:111-115` — modifies items' `AppearStart` field
  - `meta_processor_grouping.go:173-183` — modifies toc items in-place
- **Fix:** Copy the value:
  ```go
  pageCopy := *items[i].Page
  items[i].PhysicalIndex = &pageCopy
  ```

---

## LOW

### CR-021 `language.Detect()` allocates unnecessarily

- **File:** `pkg/language/detect.go:195-198`
- **Description:** The convenience function `Detect()` creates a new `Detector` every call, but `Detector` is stateless (empty struct). The allocation is unnecessary.
- **Fix:** Make `Detect` a direct function without the `Detector` indirection, or use a package-level singleton.

---

### CR-022 Magic numbers throughout codebase

- **Locations:**
  - `pkg/document/tree.go:24` — `uuid.New().String()[:12]` — Why 12 characters?
  - `pkg/indexer/meta_processor_grouping.go:28` — `tokens := len(content) / 4` — Why not use actual tokenizer?
  - `pkg/indexer/toc_extraction.go:46` — `maxAttempts = 5` — Should be config
  - `pkg/indexer/generator_summaries.go:48` — `summaryConcurrency := max(1, g.cfg.MaxConcurrency*2)` — Why \*2?
  - `pkg/indexer/meta_processor_verify.go:91` — `accuracy > 0.6` — Threshold should be config
- **Fix:** Extract to named constants or config values.

---

### CR-023 OpenAI OCR sets both `Content` and `MultiContent`

- **File:** `pkg/llm/ocr_openai.go:74-86`
- **Code:**
  ```go
  {
      Role:    openai.ChatMessageRoleUser,
      Content: "Extract all text from this image:",  // AND
      MultiContent: []openai.ChatMessagePart{...},   // Both set
  }
  ```
- **Description:** Per the OpenAI API spec, when `MultiContent` is set, `Content` should be empty. The behavior depends on the SDK implementation and may cause issues with some API providers.
- **Fix:** Move the text instruction into `MultiContent` as a text part:
  ```go
  MultiContent: []openai.ChatMessagePart{
      {Type: openai.ChatMessagePartTypeText, Text: "Extract all text from this image:"},
      {Type: openai.ChatMessagePartTypeImageURL, ...},
  }
  ```

---

### CR-024 No graceful shutdown / signal handling

- **File:** `cmd/pageindex/main.go`
- **Description:** No signal handling for SIGINT/SIGTERM. Long-running LLM operations (which can take 10+ minutes for large PDFs) can't be cleanly cancelled with Ctrl+C. The context has a 30-minute timeout but no signal-based cancellation.
- **Fix:** Use `signal.NotifyContext` to create a cancellable context:
  ```go
  ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
  defer stop()
  ```

---

### CR-025 Dead code — `toc_offset.go` functions operating on `*TOCResult`

- **File:** `pkg/indexer/toc_offset.go:12-58`
- **Description:** `calculatePageOffset(toc *TOCResult)` and `addPageOffsetToTOC(toc *TOCResult, offset int)` operate on `*TOCResult` but the actual offset calculation in the pipeline uses `meta_processor_helpers.go`'s `calculatePageOffset(pairs []PageIndexPair)` which operates on `[]PageIndexPair`.
- **Fix:** Remove the unused functions. Keep only `tocIndexExtractorPrompt` and `addPhysicalIndexToTOC` which are actively used.

---

## Summary

| Severity | Count | Estimated Lines to Change |
|----------|-------|--------------------------|
| CRITICAL | 3 | ~50 |
| HIGH | 9 | ~800 (mostly deletions) |
| MEDIUM | 8 | ~150 |
| LOW | 5 | ~40 |
| **Total** | **25** | **~1,040** |

### Priority Order

1. **P0 (Correctness):** CR-001, CR-002, CR-003 — Data races and concurrency bugs
2. **P1 (Cleanup):** CR-010, CR-011, CR-012, CR-025 — Remove ~600 lines of dead code
3. **P2 (Quality):** CR-005, CR-004, CR-007 — Reduce duplication, fix file sizes
4. **P3 (Structure):** CR-006 — Split God package
5. **P4 (Robustness):** CR-015, CR-019, CR-020 — Fix silent data corruption risks
6. **P5 (Polish):** CR-013, CR-016, CR-017, CR-022, CR-024 — Concurrency limits, logging, conventions
