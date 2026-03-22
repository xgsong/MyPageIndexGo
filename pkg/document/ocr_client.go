package document

import "context"

// OCRRequest represents a request to perform OCR on an image.
type OCRRequest struct {
	ImageData []byte // Image data in PNG format
	PageNum   int    // Page number for error reporting
}

// OCRResponse represents the response from an OCR operation.
type OCRResponse struct {
	Text  string // Extracted text
	Error string // Error message if OCR failed
}

// OCRClient is the interface for OCR providers.
// Implementations can use Tesseract, cloud OCR APIs, or local LLM models.
type OCRClient interface {
	// Recognize performs OCR on the given image data.
	// Returns the extracted text or an error.
	Recognize(ctx context.Context, req *OCRRequest) (*OCRResponse, error)

	// RecognizeBatch performs OCR on multiple images in batch.
	// More efficient than calling Recognize multiple times.
	RecognizeBatch(ctx context.Context, reqs []*OCRRequest) ([]*OCRResponse, error)
}
