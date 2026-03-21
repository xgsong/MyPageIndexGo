package document

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidPDF(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name:     "empty buffer",
			input:    []byte{},
			expected: false,
		},
		{
			name:     "short buffer less than magic number length",
			input:    []byte("%PD"),
			expected: false,
		},
		{
			name:     "buffer with wrong magic number",
			input:    []byte("NOT-PDF content"),
			expected: false,
		},
		{
			name:     "valid PDF magic number",
			input:    []byte("%PDF-1.7\ncontent here"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidPDF(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPDFParser_Parse_InvalidPDF(t *testing.T) {
	parser := NewPDFParser()

	// Test with non-PDF content
	input := bytes.NewReader([]byte("This is not a PDF file"))
	doc, err := parser.Parse(input)
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Contains(t, err.Error(), "invalid PDF file")
}

func TestPDFParser_Parse_EmptyInput(t *testing.T) {
	parser := NewPDFParser()

	// Test with empty input
	input := bytes.NewReader([]byte{})
	doc, err := parser.Parse(input)
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.Contains(t, err.Error(), "invalid PDF file")
}

func TestPDFParser_extractWithOCR_Error(t *testing.T) {
	parser := NewPDFParser()
	// OCR is not enabled in standard build, should return error
	pages, err := parser.extractWithOCR([]byte("%PDF-1.7"))
	assert.Error(t, err)
	assert.Nil(t, pages)
	assert.Contains(t, err.Error(), "OCR support not available")
}

func TestNewPDFParserWithOCR(t *testing.T) {
	parser := NewPDFParserWithOCR(true, "chi_sim")
	assert.Equal(t, true, parser.EnableOCR)
	assert.Equal(t, "chi_sim", parser.OCRLanguage)

	// Test with OCR disabled
	parser = NewPDFParserWithOCR(false, "")
	assert.Equal(t, false, parser.EnableOCR)
	assert.Equal(t, "", parser.OCRLanguage)
}

// Test that io.Reader error is properly propagated
func TestPDFParser_Parse_ReadError(t *testing.T) {
	parser := NewPDFParser()

	// Create a reader that returns error
	errReader := &errorReader{err: io.ErrUnexpectedEOF}
	doc, err := parser.Parse(errReader)
	assert.Error(t, err)
	assert.Nil(t, doc)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

// errorReader is a mock reader that returns error
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
