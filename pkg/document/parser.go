package document

import (
	"io"
	"strings"
)

// DocumentParser is the adapter interface for different document formats.
// Each implementation parses a specific format (PDF, DOCX, HTML, etc.)
// and converts it to a unified Document structure with pages,
// which is then ready for downstream indexing process.
//
// This is the Adapter pattern: different input formats -> unified output for downstream processing.
// Adding new file formats only requires implementing a new adapter.
type DocumentParser interface {
	// Parse reads from input and converts it to a unified Document structure.
	// The output Document has pages ready for indexing.
	Parse(r io.Reader) (*Document, error)
	// SupportedExtensions returns the file extensions supported by this parser.
	// Extensions should be lowercase without the leading dot.
	SupportedExtensions() []string
	// Name returns the parser name for debugging.
	Name() string
}

// Document represents a parsed document after conversion.
type Document struct {
	Pages    []Page            `json:"pages"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Page represents a single page/section in the processed document.
type Page struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// GetFullText returns the concatenated text of all pages.
func (d *Document) GetFullText() string {
	var sb strings.Builder
	for _, page := range d.Pages {
		sb.WriteString(page.Text)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// TotalPages returns the number of pages in the document.
func (d *Document) TotalPages() int {
	return len(d.Pages)
}

// ParserRegistry holds all registered document parsers by file extension.
type ParserRegistry struct {
	parsers map[string]DocumentParser
}

// NewParserRegistry creates a new empty ParserRegistry.
func NewParserRegistry() *ParserRegistry {
	return &ParserRegistry{
		parsers: make(map[string]DocumentParser),
	}
}

// Register registers a parser for the given file extension.
// Extension should be lowercase without the dot, e.g. "pdf", "md".
func (r *ParserRegistry) Register(ext string, parser DocumentParser) {
	r.parsers[ext] = parser
}

// Get gets the parser for the given file extension.
// Extension can be with or without the dot, case-insensitive.
func (r *ParserRegistry) Get(ext string) (DocumentParser, bool) {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	parser, ok := r.parsers[ext]
	return parser, ok
}

// DefaultRegistry returns the default registry with all supported formats.
func DefaultRegistry() *ParserRegistry {
	reg := NewParserRegistry()
	reg.Register("pdf", NewPDFParser())
	reg.Register("md", NewMarkdownParser())
	return reg
}
