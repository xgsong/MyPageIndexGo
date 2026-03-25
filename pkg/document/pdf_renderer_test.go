package document

import (
	"context"
	"os"
	"testing"

	"github.com/gen2brain/go-fitz"
	"github.com/stretchr/testify/assert"
)

func TestNewPDFRenderer(t *testing.T) {
	tests := []struct {
		name     string
		dpi      int
		expected float64
	}{
		{"zero dpi defaults to 150", 0, 150.0},
		{"negative dpi defaults to 150", -10, 150.0},
		{"valid dpi 72", 72, 72.0},
		{"valid dpi 150", 150, 150.0},
		{"valid dpi 300", 300, 300.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := NewPDFRenderer(tt.dpi)
			assert.NotNil(t, renderer)
			assert.Equal(t, tt.expected, renderer.dpi)
		})
	}
}

func TestNewPDFRenderer_ScaleCalculation(t *testing.T) {
	renderer := NewPDFRenderer(150)
	assert.Equal(t, 150.0/72.0, renderer.scale)
}

func TestPDFRenderer_SetQuality(t *testing.T) {
	renderer := NewPDFRenderer(150)
	renderer.SetQuality(0)
	assert.NotNil(t, renderer)
}

func TestPDFRenderer_RenderPage_InvalidPageNumber(t *testing.T) {
	renderer := NewPDFRenderer(150)

	pdfData, err := os.ReadFile("../test/fixtures/test.pdf")
	if err != nil {
		t.Skip("Test fixture not found, skipping PDF rendering tests")
	}

	doc, err := fitz.NewFromMemory(pdfData)
	if err != nil {
		t.Skip("Cannot open test PDF, skipping")
	}
	defer doc.Close()

	img, err := renderer.RenderPage(doc, -1)
	assert.Error(t, err)
	assert.Nil(t, img)

	img, err = renderer.RenderPage(doc, doc.NumPage())
	assert.Error(t, err)
	assert.Nil(t, img)
}

func TestPDFRenderer_RenderAllPages_ContextCancellation(t *testing.T) {
	renderer := NewPDFRenderer(150)

	pdfData, err := os.ReadFile("../test/fixtures/test.pdf")
	if err != nil {
		t.Skip("Test fixture not found, skipping PDF rendering tests")
	}

	doc, err := fitz.NewFromMemory(pdfData)
	if err != nil {
		t.Skip("Cannot open test PDF, skipping")
	}
	defer doc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	images, err := renderer.RenderAllPages(ctx, doc, 1)
	assert.Error(t, err)
	assert.Nil(t, images)
}

func TestPDFRenderer_RenderAllPagesFromBytes_InvalidPDF(t *testing.T) {
	renderer := NewPDFRenderer(150)

	ctx := context.Background()
	images, err := renderer.RenderAllPagesFromBytes(ctx, []byte("not a pdf"), 1)
	assert.Error(t, err)
	assert.Nil(t, images)
}

func TestPDFRenderer_RenderAllPagesFromBytes_EmptyPDF(t *testing.T) {
	renderer := NewPDFRenderer(150)

	emptyPDF := []byte(`%PDF-1.0
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [] /Count 0 >>
endobj
xref
0 3
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
trailer
<< /Size 3 /Root 1 0 R >>
startxref
0
%%EOF`)

	ctx := context.Background()
	images, err := renderer.RenderAllPagesFromBytes(ctx, emptyPDF, 1)
	assert.NoError(t, err)
	assert.Len(t, images, 0)
}

func TestPDFRenderer_RenderAllPages_Success(t *testing.T) {
	renderer := NewPDFRenderer(150)

	pdfData, err := os.ReadFile("../test/fixtures/test.pdf")
	if err != nil {
		t.Skip("Test fixture not found, skipping PDF rendering tests")
	}

	doc, err := fitz.NewFromMemory(pdfData)
	if err != nil {
		t.Skip("Cannot open test PDF, skipping")
	}
	defer doc.Close()

	ctx := context.Background()
	images, err := renderer.RenderAllPages(ctx, doc, 1)
	assert.NoError(t, err)
	assert.Len(t, images, doc.NumPage())
	for _, img := range images {
		assert.NotEmpty(t, img)
	}
}