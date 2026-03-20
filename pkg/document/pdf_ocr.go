//go:build ocr
// +build ocr

package document

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"strings"

	"github.com/gen2brain/go-fitz"
	"github.com/otiai10/gosseract/v2"
)

// extractWithOCR renders PDF pages as images and uses OCR to extract text (for scanned PDFs).
// This implementation is only included when building with the "ocr" build tag.
func (p *PDFParser) extractWithOCR(buf []byte) ([]Page, error) {
	// Open PDF with fitz for rendering
	doc, err := fitz.NewFromMemory(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF for OCR: %w", err)
	}
	defer doc.Close()

	numPages := doc.NumPage()
	pages := make([]Page, 0, numPages)

	// Initialize OCR client
	client := gosseract.NewClient()
	defer client.Close()

	if err := client.SetLanguage(p.OCRLanguage); err != nil {
		return nil, fmt.Errorf("failed to set OCR language %q: %w (ensure tessdata is installed)", p.OCRLanguage, err)
	}

	for i := 0; i < numPages; i++ {
		pageNum := i + 1

		// Render page as image (300 DPI for good OCR accuracy)
		img, err := doc.ImageDPI(i, 300.0)
		if err != nil {
			return nil, fmt.Errorf("failed to render page %d: %w", pageNum, err)
		}

		// Convert image to bytes for OCR
		var imgBuf bytes.Buffer
		if err := encodeImage(&imgBuf, img); err != nil {
			return nil, fmt.Errorf("failed to encode page %d: %w", pageNum, err)
		}

		// Run OCR
		if err := client.SetImageFromBytes(imgBuf.Bytes()); err != nil {
			return nil, fmt.Errorf("failed to load page %d to OCR: %w", pageNum, err)
		}

		text, err := client.Text()
		if err != nil {
			return nil, fmt.Errorf("OCR failed for page %d: %w", pageNum, err)
		}

		// Normalize text
		text = strings.ReplaceAll(text, "\r\n", "\n")
		text = strings.TrimSpace(text)

		pages = append(pages, Page{
			Number: pageNum,
			Text:   text,
		})
	}

	return pages, nil
}

// encodeImage converts an image to PNG format for OCR.
func encodeImage(w io.Writer, img image.Image) error {
	// Use PNG for lossless encoding (best for OCR accuracy)
	return png.Encode(w, img)
}
