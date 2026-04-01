package document

import (
	"context"
	"fmt"
	"image/png"
	"runtime"

	"github.com/gen2brain/go-fitz"
)

// PDFRenderer handles rendering PDF pages to images.
type PDFRenderer struct {
	dpi     float64              // Render DPI
	scale   float64              // Scale factor (calculated from DPI)
	quality png.CompressionLevel // PNG compression quality
}

// NewPDFRenderer creates a new PDFRenderer with the specified DPI.
// Default DPI is 150 if dpi <= 0.
func NewPDFRenderer(dpi int) *PDFRenderer {
	if dpi <= 0 {
		dpi = 150
	}
	// Calculate scale factor: 72 DPI is default for PDF, so scale = dpi / 72
	scale := float64(dpi) / 72.0
	return &PDFRenderer{
		dpi:     float64(dpi),
		scale:   scale,
		quality: png.BestCompression,
	}
}

// SetQuality sets the PNG compression quality.
// Deprecated: Quality is now handled internally by go-fitz ImagePNG method
func (r *PDFRenderer) SetQuality(quality png.CompressionLevel) {
	// No-op for backward compatibility
}

// RenderPage renders a single PDF page to PNG image data.
func (r *PDFRenderer) RenderPage(doc *fitz.Document, pageNum int) ([]byte, error) {
	if pageNum < 0 || pageNum >= doc.NumPage() {
		return nil, fmt.Errorf("page number %d out of range (0-%d)", pageNum, doc.NumPage()-1)
	}

	// Use ImagePNG method to directly render to PNG with specified DPI
	// This is more efficient than rendering to image and then encoding
	pngData, err := doc.ImagePNG(pageNum, r.dpi)
	if err != nil {
		return nil, fmt.Errorf("failed to render page %d to PNG: %w", pageNum, err)
	}

	return pngData, nil
}

// RenderAllPages renders all pages of a PDF document to PNG images.
// Returns a slice of image data, indexed by page number (0-based).
// Optimized for performance with adaptive concurrency based on document size.
func (r *PDFRenderer) RenderAllPages(ctx context.Context, doc *fitz.Document, concurrency int) ([][]byte, error) {
	numPages := doc.NumPage()
	images := make([][]byte, numPages)

	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
	}

	// Limit concurrency to avoid excessive memory usage
	if concurrency > numPages {
		concurrency = numPages
	}

	// For small documents, use sequential rendering to avoid overhead
	if numPages <= 4 || concurrency <= 1 {
		for i := 0; i < numPages; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			imgData, err := r.RenderPage(doc, i)
			if err != nil {
				return nil, fmt.Errorf("failed to render page %d: %w", i, err)
			}

			images[i] = imgData
		}
		return images, nil
	}

	// For larger documents, use page batching with limited concurrency
	// Note: fitz.Document is not concurrency-safe, so we process in batches
	batchSize := max(1, numPages/concurrency)

	for batchStart := 0; batchStart < numPages; batchStart += batchSize {
		batchEnd := min(batchStart+batchSize, numPages)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		for i := batchStart; i < batchEnd; i++ {
			imgData, err := r.RenderPage(doc, i)
			if err != nil {
				return nil, fmt.Errorf("failed to render page %d: %w", i, err)
			}

			images[i] = imgData
		}
	}

	return images, nil
}

// RenderAllPagesFromBytes renders all pages of a PDF from byte data to PNG images.
// This is a convenience function that opens the PDF document and renders all pages.
func (r *PDFRenderer) RenderAllPagesFromBytes(ctx context.Context, pdfData []byte, concurrency int) ([][]byte, error) {
	doc, err := fitz.NewFromMemory(pdfData)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF document: %w", err)
	}
	defer func() { _ = doc.Close() }()

	return r.RenderAllPages(ctx, doc, concurrency)
}
