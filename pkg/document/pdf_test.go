package document

import (
	"bytes"
	"context"
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
	// OCR is not configured in standard parser, should return error
	pages, err := parser.extractWithOCR(context.TODO(), []byte("%PDF-1.7"))
	assert.Error(t, err)
	assert.Nil(t, pages)
	assert.Contains(t, err.Error(), "OCR client not configured")
}

func TestPDFParser_WithOCRClient(t *testing.T) {
	// Create a mock OCR client
	mockOCR := &mockOCRClient{}
	parser := NewPDFParserWithOCR(mockOCR)

	// Verify OCR is enabled
	assert.True(t, parser.enableOCR)
	assert.NotNil(t, parser.ocrClient)
}

func TestPDFParser_WithNilOCRClient(t *testing.T) {
	// Test with nil OCR client
	parser := NewPDFParserWithOCR(nil)

	// Verify OCR is disabled when client is nil
	assert.False(t, parser.enableOCR)
	assert.Nil(t, parser.ocrClient)
}

// mockOCRClient is a mock implementation of OCRClient for testing
type mockOCRClient struct{}

func (m *mockOCRClient) Recognize(ctx context.Context, req *OCRRequest) (*OCRResponse, error) {
	return &OCRResponse{Text: "mock text"}, nil
}

func (m *mockOCRClient) RecognizeBatch(ctx context.Context, reqs []*OCRRequest) ([]*OCRResponse, error) {
	responses := make([]*OCRResponse, len(reqs))
	for i := range reqs {
		responses[i] = &OCRResponse{Text: "mock text"}
	}
	return responses, nil
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
