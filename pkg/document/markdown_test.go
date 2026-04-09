package document

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarkdownParser_NewMarkdownParser(t *testing.T) {
	parser := NewMarkdownParser()
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.md)
}

func TestMarkdownParser_Parse_Content(t *testing.T) {
	parser := NewMarkdownParser()

	content := `# Main Title

This is a paragraph.

## Section 1

Some content here.

### Subsection 1.1

More content.
`
	reader := strings.NewReader(content)
	doc, err := parser.Parse(context.Background(), reader)

	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Len(t, doc.Pages, 1)
	assert.Equal(t, 1, doc.Pages[0].Number)
	assert.Contains(t, doc.Pages[0].Text, "Main Title")
	assert.Contains(t, doc.Pages[0].Text, "Section 1")
}

func TestMarkdownParser_Parse_EmptyContent(t *testing.T) {
	parser := NewMarkdownParser()

	reader := strings.NewReader("")
	doc, err := parser.Parse(context.Background(), reader)

	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Len(t, doc.Pages, 1)
	assert.Equal(t, "", doc.Pages[0].Text)
}

func TestMarkdownParser_Parse_WithCodeBlocks(t *testing.T) {
	parser := NewMarkdownParser()

	content := `# Title

Paragraph before code.

` + "```python" + `
def hello():
    print("Hello")
` + "```" + `

Paragraph after code.
`
	reader := strings.NewReader(content)
	doc, err := parser.Parse(context.Background(), reader)

	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Contains(t, doc.Pages[0].Text, "Title")
}

func TestMarkdownParser_Extensions(t *testing.T) {
	parser := NewMarkdownParser()
	exts := parser.SupportedExtensions()

	assert.Len(t, exts, 3)
	assert.Contains(t, exts, "md")
	assert.Contains(t, exts, "markdown")
	assert.Contains(t, exts, "txt")
}

func TestMarkdownParser_ParserName(t *testing.T) {
	parser := NewMarkdownParser()
	assert.Equal(t, "Markdown", parser.Name())
}

func TestMarkdownParser_Parse_WithLists(t *testing.T) {
	parser := NewMarkdownParser()

	content := `# Title

- Item 1
- Item 2
- Item 3
`
	reader := strings.NewReader(content)
	doc, err := parser.Parse(context.Background(), reader)

	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Contains(t, doc.Pages[0].Text, "Item 1")
	assert.Contains(t, doc.Pages[0].Text, "Item 2")
}
