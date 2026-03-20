package document

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

var pdfMagicNumber = []byte("%PDF-")

const maxPDFFileSizeBytes = 50 * 1024 * 1024

// PDFParser implements DocumentParser for PDF files.
// Supports both text-based PDFs (text extraction) and scanned PDFs (OCR, requires build tag "ocr").
type PDFParser struct {
	EnableOCR   bool   // Whether to enable OCR for scanned PDFs (requires OCR build)
	OCRLanguage string // OCR language code (default: "eng" for English, use "chi_sim" for Simplified Chinese)
}

// NewPDFParser creates a new PDFParser with default settings.
// OCR is disabled by default in standard builds, enable with build tag "ocr".
func NewPDFParser() *PDFParser {
	return &PDFParser{
		EnableOCR:   false,
		OCRLanguage: "eng",
	}
}

// NewPDFParserWithOCR creates a new PDFParser with explicit OCR settings.
// Note: OCR functionality requires building with the "ocr" build tag.
func NewPDFParserWithOCR(enableOCR bool, language string) *PDFParser {
	return &PDFParser{
		EnableOCR:   enableOCR,
		OCRLanguage: language,
	}
}

func isValidPDF(buf []byte) bool {
	if len(buf) < len(pdfMagicNumber) {
		return false
	}
	return bytes.HasPrefix(buf, pdfMagicNumber)
}

// Parse parses a PDF document and returns unified document.
// First attempts to extract text layer, falls back to OCR if text is empty and OCR is enabled.
func (p *PDFParser) Parse(r io.Reader) (*Document, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF: %w", err)
	}

	if len(buf) > maxPDFFileSizeBytes {
		return nil, fmt.Errorf("PDF file too large: %d bytes exceeds maximum of %d bytes", len(buf), maxPDFFileSizeBytes)
	}

	if !isValidPDF(buf) {
		return nil, fmt.Errorf("invalid PDF file: file does not start with PDF magic number %%PDF-")
	}

	pages, err := p.extractTextLayer(buf)
	if err != nil {
		return nil, err
	}

	hasText := false
	for _, page := range pages {
		if strings.TrimSpace(page.Text) != "" {
			hasText = true
			break
		}
	}

	if !hasText && p.EnableOCR {
		pages, err = p.extractWithOCR(buf)
		if err != nil {
			return nil, fmt.Errorf("OCR failed: %w", err)
		}
	}

	return &Document{
		Pages:    pages,
		Metadata: make(map[string]string),
	}, nil
}

// extractTextLayer extracts text from the PDF's text layer (for text-based PDFs)
func (p *PDFParser) extractTextLayer(buf []byte) ([]Page, error) {
	reader := bytes.NewReader(buf)
	pdfReader, err := pdf.NewReader(reader, int64(len(buf)))
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF for text extraction: %w", err)
	}

	numPages := pdfReader.NumPage()
	pages := make([]Page, 0, numPages)

	fonts := make(map[string]*pdf.Font)
	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			pages = append(pages, Page{Number: i, Text: ""})
			continue
		}

		text, err := page.GetPlainText(fonts)
		if err != nil {
			return nil, fmt.Errorf("failed to extract text from page %d: %w", i, err)
		}

		pages = append(pages, Page{
			Number: i,
			Text:   text,
		})
	}

	return pages, nil
}

// extractWithOCR is a stub for non-OCR builds.
// When built without "ocr" tag, returns error indicating OCR support is not compiled in.
func (p *PDFParser) extractWithOCR(buf []byte) ([]Page, error) {
	return nil, fmt.Errorf("OCR support not available: rebuild with -tags ocr to enable scanned PDF support (requires Tesseract installed)")
}

// SupportedExtensions returns the file extensions supported by this parser.
func (p *PDFParser) SupportedExtensions() []string {
	return []string{"pdf"}
}

// Name returns the parser name.
func (p *PDFParser) Name() string {
	return "PDF"
}
