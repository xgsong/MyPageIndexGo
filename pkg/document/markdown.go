package document

import (
	"fmt"
	"io"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// MarkdownParser implements DocumentParser for Markdown files.
// Adapter pattern: already Markdown, just extract text.
type MarkdownParser struct {
	md goldmark.Markdown
}

// NewMarkdownParser creates a new MarkdownParser.
func NewMarkdownParser() *MarkdownParser {
	return &MarkdownParser{
		md: goldmark.New(),
	}
}

// Parse parses a Markdown document.
// The entire Markdown document becomes a single page in the output.
func (p *MarkdownParser) Parse(r io.Reader) (*Document, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read Markdown: %w", err)
	}

	reader := text.NewReader(content)
	doc := p.md.Parser().Parse(reader)

	// Extract all text
	var fullText strings.Builder
	walker := &textExtractor{
		buf:    &fullText,
		source: content,
	}

	err = ast.Walk(doc, walker.walk)
	if err != nil {
		return nil, fmt.Errorf("failed to extract text: %w", err)
	}

	// For Markdown, entire document is one page
	// If future support for large Markdown needs paging, it can be added later
	page := Page{
		Number: 1,
		Text:   fullText.String(),
	}

	return &Document{
		Pages:    []Page{page},
		Metadata: make(map[string]string),
	}, nil
}

// SupportedExtensions returns the file extensions supported by this parser.
func (p *MarkdownParser) SupportedExtensions() []string {
	return []string{"md", "markdown", "txt"}
}

// Name returns the parser name.
func (p *MarkdownParser) Name() string {
	return "Markdown"
}

// textExtractor helps extract plain text from Markdown AST.
type textExtractor struct {
	buf    *strings.Builder
	source []byte
}

func (te *textExtractor) walk(node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	switch n := node.(type) {
	case *ast.Text:
		te.buf.Write(n.Text(te.source))
		te.buf.WriteByte(' ')
	case *ast.Heading:
		// Add newlines before headings for better separation
		te.buf.WriteString("\n\n")
	}

	return ast.WalkContinue, nil
}
