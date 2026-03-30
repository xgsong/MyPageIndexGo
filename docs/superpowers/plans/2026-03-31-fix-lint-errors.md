# Fix All Lint Errors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all 24 lint errors in the project to achieve zero lint error status ✅ **COMPLETED**

**Architecture:** Fix errors by file, starting with test files (can use nolint comments), then production code (proper error handling)

**Tech Stack:** Go, golangci-lint, errcheck, unused linters

**Status:** ✅ All tasks completed - Zero lint errors achieved!

---

## Files to Fix

### Test Files (use nolint comments for cleanup code)
- `pkg/config/config_test.go` - 6 errors (os.Setenv/Unsetenv)
- `pkg/document/integration_test.go` - 4 errors (file.Close, os.Remove)
- `pkg/document/pdf_renderer_test.go` - 1 error (doc.Close)
- `test/e2e/e2e_test.go` - 4 errors (os.Remove, file.Close)

### Production Files (proper error handling)
- `pkg/progress/tracker.go` - 6 errors (progressbar methods + unused field)
- `cmd/pageindex/main.go` - 3 errors (progressbar methods)

---

### Task 1: Fix pkg/config/config_test.go (6 errors)

**Files:**
- Modify: `pkg/config/config_test.go:26-52`

- [ ] **Step 1: Read the file to understand context**

Read lines 20-60 to see the test structure

- [ ] **Step 2: Fix os.Setenv/Unsetenv errors with proper error checking**

```go
// Line 26: Check error
origKey := os.Getenv("OPENAI_API_KEY")
if err := os.Setenv("OPENAI_API_KEY", "test-key-123"); err != nil {
    t.Fatalf("Failed to set OPENAI_API_KEY: %v", err)
}

// Lines 30-32: Check errors in cleanup
defer func() {
    if err := os.Setenv("OPENAI_API_KEY", origKey); err != nil {
        t.Logf("Warning: Failed to restore OPENAI_API_KEY: %v", err)
    }
}()

// Similar pattern for other occurrences
```

- [ ] **Step 3: Run lint to verify fixes**

Run: `golangci-lint run ./pkg/config/...`
Expected: 0 errors for config_test.go

- [ ] **Step 4: Run tests to ensure no regression**

Run: `go test ./pkg/config/... -v`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add pkg/config/config_test.go
git commit -m "fix: check os.Setenv/Unsetenv errors in config tests"
```

---

### Task 2: Fix pkg/document/integration_test.go (4 errors)

**Files:**
- Modify: `pkg/document/integration_test.go:21-105`

- [ ] **Step 1: Read the file to understand context**

Read lines 15-110 to see the test structure

- [ ] **Step 2: Add nolint comments for cleanup code**

```go
// Line 21: Add nolint
defer file.Close() // nolint:errcheck // Test cleanup, ignore errors

// Line 60: Add nolint
defer file.Close() // nolint:errcheck // Test cleanup, ignore errors

// Line 74: Add nolint
defer os.Remove(tmpFile.Name()) // nolint:errcheck // Test cleanup, ignore errors

// Line 105: Add nolint
defer file.Close() // nolint:errcheck // Test cleanup, ignore errors
```

- [ ] **Step 3: Run lint to verify fixes**

Run: `golangci-lint run ./pkg/document/...`
Expected: 0 errors for integration_test.go

- [ ] **Step 4: Run tests to ensure no regression**

Run: `go test ./pkg/document/... -v`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add pkg/document/integration_test.go
git commit -m "fix: add nolint comments for cleanup code in document tests"
```

---

### Task 3: Fix pkg/document/pdf_renderer_test.go (1 error)

**Files:**
- Modify: `pkg/document/pdf_renderer_test.go:138`

- [ ] **Step 1: Read the file to understand context**

Read lines 130-145 to see the test structure

- [ ] **Step 2: Add nolint comment for cleanup code**

```go
// Line 138: Add nolint
defer doc.Close() // nolint:errcheck // Test cleanup, ignore errors
```

- [ ] **Step 3: Run lint to verify fix**

Run: `golangci-lint run ./pkg/document/...`
Expected: 0 errors for pdf_renderer_test.go

- [ ] **Step 4: Commit**

```bash
git add pkg/document/pdf_renderer_test.go
git commit -m "fix: add nolint comment for doc.Close in pdf renderer test"
```

---

### Task 4: Fix test/e2e/e2e_test.go (4 errors)

**Files:**
- Modify: `test/e2e/e2e_test.go:84-85, 196-197`

- [ ] **Step 1: Read the file to understand context**

Read lines 80-100 and 190-200 to see the test structure

- [ ] **Step 2: Add nolint comments for cleanup code**

```go
// Lines 84-85: Add nolint
defer os.Remove(tmpPath) // nolint:errcheck // Test cleanup, ignore errors
tmpFile.Close()          // nolint:errcheck // Test cleanup, ignore errors

// Lines 196-197: Add nolint
defer os.Remove(tmpPath) // nolint:errcheck // Test cleanup, ignore errors
tmpFile.Close()          // nolint:errcheck // Test cleanup, ignore errors
```

- [ ] **Step 3: Run lint to verify fixes**

Run: `golangci-lint run ./test/e2e/...`
Expected: 0 errors for e2e_test.go

- [ ] **Step 4: Run tests to ensure no regression**

Run: `go test ./test/e2e/... -v`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add test/e2e/e2e_test.go
git commit -m "fix: add nolint comments for cleanup code in e2e tests"
```

---

### Task 5: Fix pkg/progress/tracker.go (6 errors)

**Files:**
- Modify: `pkg/progress/tracker.go:20-145`

- [ ] **Step 1: Read the file to understand context**

Read the entire file to understand the progressbar usage

- [ ] **Step 2: Remove unused field**

```go
// Line 74: Remove unused field
type MultiStepTracker struct {
    // description string  // REMOVE THIS LINE
    // ... other fields
}
```

- [ ] **Step 3: Add nolint comments for progressbar methods**

Progressbar methods returning errors is acceptable to ignore in production code
since progress bar is non-critical UI feedback

```go
// Line 20: Add nolint
p.bar.Add(n) // nolint:errcheck // Progress bar error non-critical

// Line 24: Add nolint
p.bar.Set64(current) // nolint:errcheck // Progress bar error non-critical

// Line 32: Add nolint
p.bar.Finish() // nolint:errcheck // Progress bar error non-critical

// Line 120: Add nolint
mst.bar.Set64(currentVal) // nolint:errcheck // Progress bar error non-critical

// Line 145: Add nolint
mst.bar.Finish() // nolint:errcheck // Progress bar error non-critical
```

- [ ] **Step 4: Run lint to verify fixes**

Run: `golangci-lint run ./pkg/progress/...`
Expected: 0 errors

- [ ] **Step 5: Run tests to ensure no regression**

Run: `go test ./pkg/progress/... -v`
Expected: All tests pass (note: tests may not exist yet)

- [ ] **Step 6: Commit**

```bash
git add pkg/progress/tracker.go
git commit -m "fix: remove unused field and add nolint for progressbar methods"
```

---

### Task 6: Fix cmd/pageindex/main.go (3 errors)

**Files:**
- Modify: `cmd/pageindex/main.go:220-233`

- [ ] **Step 1: Read the file to understand context**

Read lines 215-240 to see the progressbar usage

- [ ] **Step 2: Add nolint comments for progressbar methods**

```go
// Line 220: Add nolint
bar.Set(percent) // nolint:errcheck // Progress bar error non-critical

// Line 224: Add nolint
bar.Set(5) // nolint:errcheck // Progress bar error non-critical

// Line 233: Add nolint
bar.Finish() // nolint:errcheck // Progress bar error non-critical
```

- [ ] **Step 3: Run lint to verify fixes**

Run: `golangci-lint run ./cmd/pageindex/...`
Expected: 0 errors

- [ ] **Step 4: Run tests to ensure no regression**

Run: `go test ./cmd/pageindex/... -v`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add cmd/pageindex/main.go
git commit -m "fix: add nolint comments for progressbar methods in main"
```

---

### Task 7: Final Verification

**Files:**
- All modified files

- [ ] **Step 1: Run full lint check**

Run: `make lint` or `golangci-lint run ./...`
Expected: 0 errors, clean output

- [ ] **Step 2: Run all tests**

Run: `make test` or `go test ./... -race -cover`
Expected: All tests pass, coverage maintained or improved

- [ ] **Step 3: Build the project**

Run: `make build` or `go build -o pageindex ./cmd/pageindex/`
Expected: Successful build, binary created

- [ ] **Step 4: Final commit if needed**

```bash
git status
# Verify all changes are committed
```

---

## Summary

**Total Errors Fixed:** 24
- Test files: 15 errors (using nolint comments for cleanup code)
- Production files: 9 errors (6 progressbar + 1 unused field + 2 progressbar in main)

**Approach:**
- Test files: Use `// nolint:errcheck` for cleanup code (defer statements)
- Production progressbar: Use `// nolint:errcheck` since progress bar is non-critical UI
- Remove unused fields to satisfy `unused` linter

**Expected Outcome:**
- Zero lint errors
- All tests passing
- No functional changes
- Clean `make lint` output
