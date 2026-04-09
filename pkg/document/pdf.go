package document

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
	"github.com/rs/zerolog/log"
)

var pdfMagicNumber = []byte("%PDF-")

const maxPDFFileSizeBytes = 50 * 1024 * 1024

// PDFParser implements DocumentParser for PDF files.
// Supports both text-based PDFs (text extraction) and scanned PDFs (OCR via LLM API).
type PDFParser struct {
	ocrClient OCRClient // Optional OCR client for scanned PDFs
	enableOCR bool      // Whether to enable OCR fallback
}

// NewPDFParser creates a new PDFParser with default settings (OCR disabled).
func NewPDFParser() *PDFParser {
	return &PDFParser{
		enableOCR: false,
	}
}

// NewPDFParserWithOCR creates a new PDFParser with OCR client.
func NewPDFParserWithOCR(ocrClient OCRClient) *PDFParser {
	return &PDFParser{
		ocrClient: ocrClient,
		enableOCR: ocrClient != nil,
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
// Context is used for cancellation during OCR operations.
func (p *PDFParser) Parse(ctx context.Context, r io.Reader) (*Document, error) {
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

	// Count empty pages to detect mixed-content PDFs
	// Trigger OCR if all pages are empty or more than 30% are empty
	hasText := false
	emptyPageCount := 0
	for _, page := range pages {
		if strings.TrimSpace(page.Text) != "" {
			hasText = true
		} else {
			emptyPageCount++
		}
	}

	if p.ocrClient != nil && (!hasText || emptyPageCount > len(pages)*3/10) {
		log.Info().
			Int("total_pages", len(pages)).
			Int("empty_pages", emptyPageCount).
			Bool("has_text", hasText).
			Msg("Triggering OCR for mixed-content PDF")
		pages, err = p.extractWithOCR(ctx, buf)
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

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			pages = append(pages, Page{Number: i, Text: ""})
			continue
		}

		// Try GetTextByRow first for better text extraction
		text, err := extractTextFromPage(page)
		if err != nil {
			// Fallback to GetPlainText if GetTextByRow fails
			text, err = extractTextWithPlainText(page)
			if err != nil {
				return nil, fmt.Errorf("failed to extract text from page %d: %w", i, err)
			}
		}

		// Clean up the extracted text
		text = cleanExtractedText(text)

		pages = append(pages, Page{
			Number: i,
			Text:   text,
		})
	}

	return pages, nil
}

// extractTextFromPage uses GetTextByRow for better text extraction
func extractTextFromPage(page pdf.Page) (string, error) {
	rows, err := page.GetTextByRow()
	if err != nil {
		return "", err
	}

	// Pre-calculate approximate size for efficiency
	totalLen := 0
	for _, row := range rows {
		for _, word := range row.Content {
			totalLen += len(word.S)
		}
		totalLen += 1 // for newline
	}

	var textBuilder strings.Builder
	if totalLen > 0 {
		textBuilder.Grow(totalLen)
	}
	for _, row := range rows {
		for _, word := range row.Content {
			textBuilder.WriteString(word.S)
		}
		textBuilder.WriteString("\n")
	}

	return textBuilder.String(), nil
}

// extractTextWithPlainText uses GetPlainText as fallback
func extractTextWithPlainText(page pdf.Page) (string, error) {
	fonts := make(map[string]*pdf.Font)
	text, err := page.GetPlainText(fonts)
	if err != nil {
		return "", err
	}
	return text, nil
}

// cleanExtractedText removes non-printable characters and normalizes text
func cleanExtractedText(text string) string {
	// Pre-allocate with same size as input (output will be <= input)
	var result strings.Builder
	result.Grow(len(text))

	for _, r := range text {
		// Keep printable characters and common whitespace
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// extractWithOCR renders PDF pages as images and uses OCR API to extract text (for scanned PDFs).
func (p *PDFParser) extractWithOCR(ctx context.Context, buf []byte) ([]Page, error) {
	if p.ocrClient == nil {
		return nil, fmt.Errorf("OCR client not configured")
	}

	// Create PDF renderer with default DPI 150
	// TODO: Make DPI configurable via PDFParser options
	renderer := NewPDFRenderer(150)

	// Render all pages to images
	// TODO: Make concurrency configurable via PDFParser options
	images, err := renderer.RenderAllPagesFromBytes(ctx, buf, 5)
	if err != nil {
		return nil, fmt.Errorf("failed to render PDF pages: %w", err)
	}

	// Prepare OCR requests
	reqs := make([]*OCRRequest, len(images))
	for i, imgData := range images {
		reqs[i] = &OCRRequest{
			ImageData: imgData,
			PageNum:   i + 1, // Page numbers are 1-based
		}
	}

	// Perform OCR recognition
	responses, err := p.ocrClient.RecognizeBatch(ctx, reqs)
	if err != nil {
		return nil, fmt.Errorf("OCR recognition failed: %w", err)
	}

	// Convert OCR responses to Pages
	pages := make([]Page, len(responses))
	for i, resp := range responses {
		if resp.Error != "" {
			return nil, fmt.Errorf("OCR failed for page %d: %s", resp.PageNum, resp.Error)
		}

		pages[i] = Page{
			Number: resp.PageNum,
			Text:   resp.Text,
			// TODO: Store structured OCR data in Page metadata when Page struct supports it
		}
	}

	return pages, nil
}

// SupportedExtensions returns the file extensions supported by this parser.
func (p *PDFParser) SupportedExtensions() []string {
	return []string{"pdf"}
}

// Name returns the parser name.
func (p *PDFParser) Name() string {
	return "PDF"
}
