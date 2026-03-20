# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PageIndex is an LLM-based **vectorless, reasoning-based RAG** system. Key features:
- **No Vector DB**: Uses document structure and LLM reasoning for retrieval, instead of vector similarity search
- **No Chunking**: Documents are organized into natural semantic sections, not artificial chunks
- **Human-like Retrieval**: Simulates how human experts navigate and extract knowledge from complex documents
- **Better Explainability**: Retrieval is reasoning-based, fully traceable with page and section references
- Achieves **98.7% accuracy** on FinanceBench dataset, outperforming traditional vector-based RAG

PageIndex performs retrieval in two steps:
1. Generate a hierarchical "Table-of-Contents" tree structure index from documents
2. Perform reasoning-based retrieval through tree search

This is the Go port of the original Python implementation ([VectifyAI/PageIndex](https://github.com/VectifyAI/PageIndex)).

**For detailed technical specifications, see [TECH_SPEC.md](./TECH_SPEC.md).**

## Commands

### Dependencies
```bash
go mod tidy              # Download and tidy dependencies
go list -m all           # List all dependencies
go get <package>         # Add new dependency
go get -u <package>      # Update dependency
```

### Formatting
```bash
go fmt ./...             # Auto-format all Go files
gofumpt -w ./...         # Enhanced formatting (optional)
go vet ./...             # Run go vet to check for issues
```

### Build
```bash
go build -o pageindex ./cmd/pageindex
go build -race -o pageindex ./cmd/pageindex  # Build with race detector
```

### Run
```bash
./pageindex --help
./pageindex generate --pdf /path/to/document.pdf    # Generate index from PDF
./pageindex generate --md /path/to/document.md     # Generate index from Markdown
./pageindex search --index index.json --query "your question"  # Search the index
```

### Supported CLI Commands

- `generate` - Generate PageIndex tree from document
  - `--pdf` - Path to PDF file
  - `--md` - Path to Markdown file
  - `--output` - Output JSON file path (default: output.json)
  - `--model` - OpenAI model to use (default: gpt-4o)
  - `--max-concurrency` - Maximum concurrent LLM calls

- `search` - Search the generated index with a query
  - `--index` - Path to generated index JSON
  - `--query` - Search query
  - `--output` - Output JSON for results

### Test
```bash
go test ./...
go test -v ./pkg/document  # Single package test
go test -race ./...        # Run tests with race detector
```

### Clean
```bash
rm -f pageindex          # Remove binary only
```

## Dependencies

All dependencies are managed via Go modules. Go 1.25+ required.

Key dependencies:
- `github.com/pdfcpu/pdfcpu` - PDF processing (pure Go, no CGO)
- `github.com/yuin/goldmark` - Markdown processing
- `github.com/sashabaranov/go-openai` - OpenAI API client
- `github.com/pkoukk/tiktoken-go` - Token counting
- `github.com/spf13/viper` - Configuration management
- `github.com/urfave/cli/v2` - CLI framework
- `golang.org/x/sync/errgroup` - Concurrency control with error propagation

## Configuration

Configuration is managed via Viper:
- Supports YAML, JSON, and environment variables
- Environment variables prefixed with `PAGEINDEX_`

Required configuration:
```bash
export PAGEINDEX_OPENAI_API_KEY=your_openai_api_key_here
```

Optional configuration:
- `PAGEINDEX_OPENAI_BASE_URL` - Custom OpenAI base URL (for self-hosted models)
- `PAGEINDEX_MAX_CONCURRENCY` - Maximum concurrent LLM calls (default: 5)
- `PAGEINDEX_MAX_TOKENS_PER_NODE` - Maximum tokens per grouped chunk (default: 16000)
- `PAGEINDEX_GENERATE_SUMMARIES` - Whether to generate summaries for all nodes (true/false, default: false)
- `PAGEINDEX_LOG_LEVEL` - Log level (debug, info, warn, error)

## Architecture

### Project Structure

```
mypageindexgo/
├── cmd/
│   └── pageindex/
│       └── main.go         # CLI entry point
├── pkg/
│   ├── config/             # Configuration handling
│   │   └── config.go
│   ├── document/           # Document processing core
│   │   ├── parser.go       # Parser interface
│   │   ├── pdf.go          # PDF parser
│   │   ├── markdown.go     # Markdown parser
│   │   └── tree.go         # Index tree data structures
│   ├── llm/                # LLM client abstraction
│   │   ├── client.go       # LLM interface
│   │   ├── openai.go       # OpenAI implementation
│   │   └── prompts.go      # Prompt templates
│   ├── tokenizer/          # Token counting
│   │   └── tokenizer.go
│   ├── indexer/            # Index generation and search
│   │   ├── generator.go    # Directory tree generation
│   │   ├── processor.go    # Node processing
│   │   └── search.go       # Reasoning-based retrieval
│   └── output/             # Output handling
│       └── json.go         # JSON output
├── internal/
│   └── utils/              # Private utilities
│       ├── json.go         # JSON utilities
│       └── errors.go       # Error handling helpers
└── test/
    └── fixtures/           # Test documents
```

### Core Interfaces

1. **DocumentParser** - Document parsing abstraction
   - Supports PDF and Markdown out of the box
   - Returns parsed `Document` with pages and metadata

2. **LLMClient** - LLM client abstraction
   - `GenerateStructure` - Generate hierarchical node structure from text
   - `GenerateSummary` - Generate summary for a node
   - `Search` - Perform reasoning-based search on the index tree
   - Extensible to other LLM providers beyond OpenAI

3. **IndexTree** - Complete hierarchical document index
   - Root `Node` contains nested children
   - Each node has title, page range, optional summary

### Concurrency Model

Uses `goroutine + errgroup.Group` for concurrent LLM calls:

```go
// Example concurrency pattern
group, ctx := errgroup.WithContext(ctx)
group.SetLimit(cfg.MaxConcurrency)

for _, pageGroup := range pageGroups {
    pageGroup := pageGroup
    group.Go(func() error {
        node, err := llmClient.GenerateStructure(ctx, pageGroup.Text)
        if err != nil {
            return fmt.Errorf("failed to generate structure: %w", err)
        }
        // Merge node to result tree
        return nil
    })
}

if err := group.Wait(); err != nil {
    return nil, err
}
```

- `errgroup.SetLimit()` controls max concurrency to avoid API rate limiting
- Lightweight goroutines (KB stack) vs Python asyncio (MB stack)
- Native error propagation through errgroup

### Design Principles

- **Pure Go first**: No CGO for easier cross-compilation (using pdfcpu instead of go-fitz)
- **Interface-based**: Abstractions allow swapping implementations (DocumentParser, LLMClient)
- **JSON robustness**: Must handle non-standard JSON output from LLMs with proper cleanup
- **Explicit error handling**: Go-style `(result, error)` returns with `%w` wrapping
- **Immutability where possible**: Core data structures are immutable after construction
- **Concurrency safety**: Goroutine-safe design with proper synchronization

## Code Style

Follow standard Go conventions:
- **Formatting**: Always run `go fmt ./...` before committing
- **Comments**: All public identifiers must have proper Go doc comments
- **Package names**: Short, lowercase, single word (no underscores or mixedCase)
- **Error handling**: Explicit `(result, error)` returns, wrap with `fmt.Errorf("...: %w", err)`
- **File size**: Keep files under 400 lines, prefer small focused packages
- **Immutability**: Core data structures should be immutable after construction

## Debugging & Profiling

```bash
# CPU and memory profiling example
go test -cpuprofile cpu.prof -memprofile mem.prof ./pkg/document
go tool pprof cpu.prof
```

Set `PAGEINDEX_LOG_LEVEL` environment variable for debug logging.
