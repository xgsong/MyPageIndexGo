package document

import "context"

// OCRBlock represents a single recognized block of content in an image.
type OCRBlock struct {
	Type        string  `json:"type"`        // Type of block: "text", "heading", "table", "image", "list"
	Text        string  `json:"text"`        // Recognized text content
	BoundingBox [4]int  `json:"boundingBox"` // Coordinates: [x1, y1, x2, y2]
	Confidence  float64 `json:"confidence"`  // Recognition confidence (0.0 - 1.0)
	Level       int     `json:"level"`       // Heading level (for heading type)
}

// OCRRequest represents a request to perform OCR on an image.
type OCRRequest struct {
	ImageData []byte // Image data in PNG/JPG format
	PageNum   int    // Page number for error reporting
}

// OCRResponse represents the response from an OCR operation.
type OCRResponse struct {
	Text       string                 `json:"text"`       // Full plain text content
	Structured map[string]interface{} `json:"structured"` // Raw structured data from OCR provider
	Blocks     []OCRBlock             `json:"blocks"`     // List of recognized content blocks
	Confidence float64                `json:"confidence"` // Overall recognition confidence
	PageNum    int                    `json:"pageNum"`    // Corresponding page number
	Error      string                 `json:"error"`      // Error message if OCR failed
}

// OCRClient is the interface for OCR providers.
// Implementations can use Llama.cpp, Tesseract, cloud OCR APIs, or local LLM models.
type OCRClient interface {
	// Recognize performs OCR on the given image data.
	// Returns the extracted text or an error.
	Recognize(ctx context.Context, req *OCRRequest) (*OCRResponse, error)

	// RecognizeBatch performs OCR on multiple images in batch.
	// More efficient than calling Recognize multiple times.
	RecognizeBatch(ctx context.Context, reqs []*OCRRequest) ([]*OCRResponse, error)
}
