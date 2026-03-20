# PageIndex Go

> Go port of [VectifyAI/PageIndex](https://github.com/VectifyAI/PageIndex) - LLM-based **vectorless, reasoning-based RAG** system.

## Overview

PageIndex is a revolutionary RAG approach that doesn't require:
- **No vector database**
- **No text chunking**
- **No embeddings**

Instead, PageIndex:
1. Generates a hierarchical table-of-contents tree structure from your documents using LLM
2. Performs reasoning-based retrieval through tree navigation
3. Achieves **98.7% accuracy** on FinanceBench dataset, outperforming traditional vector-based RAG

## Features

- Pure Go implementation, single static binary distribution
- No CGO required, easy cross-compilation
- Supports PDF and Markdown out of the box
- Concurrent LLM processing with configurable rate limiting
- Environment-based configuration with .env support
- Clean CLI interface

## Installation

### Download Prebuilt Binary

Download the latest release from [Releases](https://github.com/xgsong/mypageindexgo/releases) for your platform:

- Linux amd64
- macOS amd64/arm64
- Windows amd64

### Build from Source

```bash
git clone https://github.com/xgsong/mypageindexgo.git
cd mypageindexgo
go build -o pageindex ./cmd/pageindex
```

Or with Make:

```bash
make build
```

## Configuration

Set your OpenAI API key in a `.env` file or environment variable:

```bash
export PAGEINDEX_OPENAI_API_KEY=your_openai_api_key_here
```

Optional configuration:
```bash
export PAGEINDEX_OPENAI_BASE_URL=https://your-custom-base-url.com/  # For self-hosted models
export PAGEINDEX_OPENAI_MODEL=gpt-4o                               # Default: gpt-4o
export PAGEINDEX_MAX_CONCURRENCY=5                                  # Default: 5
export PAGEINDEX_MAX_TOKENS_PER_NODE=16000                          # Default: 16000
export PAGEINDEX_GENERATE_SUMMARIES=false                            # Default: false
export PAGEINDEX_LOG_LEVEL=info                                      # Default: info
```

## Usage

### Generate Index

```bash
# From PDF
./pageindex generate --pdf document.pdf --output index.json

# From Markdown
./pageindex generate --md document.md --output index.json

# Custom model and concurrency
./pageindex generate --pdf document.pdf --model gpt-4o-mini --max-concurrency 10
```

### Search

```bash
./pageindex search --index index.json --query "What is the total revenue in 2023?"
```

Options:
- `--output result.json` - Save search result to JSON

## Example

```bash
# Generate index
$ ./pageindex generate --pdf example.pdf --output example.json
Parsing document: example.pdf...
Parsed 50 pages
Generating index...
Saving index to example.json...

✓ Index generation complete!
  • Total pages: 50
  • Total nodes: 12
  • Time elapsed: 30.5s
  • Output saved to: example.json

# Search
$ ./pageindex search --index example.json --query "What is the growth rate?"
Loading index from example.json...
Loaded index with 12 nodes
Searching for: What is the growth rate?

✓ Search complete! (elapsed: 2.3s)

Query: What is the growth rate?

Answer:
The company reported a 15% year-over-year growth rate in 2023, up from 12% in 2022.

Referenced nodes:
  1. Financial Performance Summary (pages 15-16)
  2. Revenue Analysis (pages 12-14)
```

## Architecture

```
mypageindexgo/
├── cmd/
│   └── pageindex/
│       └── main.go         # CLI entry point
├── pkg/
│   ├── config/             # Configuration handling
│   ├── document/           # Document parsing (PDF/Markdown)
│   ├── llm/                # LLM client abstraction
│   ├── tokenizer/          # Token counting with tiktoken
│   ├── indexer/            # Index generation and search
│   └── output/             # JSON output handling
└── internal/
    └── utils/              # JSON cleaning and error helpers
```

### Design Principles

- **Interface-based**: Easy to extend with new document formats and LLM providers
- **Concurrent**: goroutines + errgroup for efficient parallel processing
- **Immutable**: Core data structures immutable after creation
- **Pure Go**: No CGO dependencies for easy cross-compilation

## Performance

Compared to the original Python implementation:
- Faster startup time
- Lower memory usage
- Better concurrent throughput
- Single binary distribution

## License

MIT License - see LICENSE file for details.

## Acknowledgments

This is a Go port of the original [PageIndex](https://github.com/VectifyAI/PageIndex) project by VectifyAI.
