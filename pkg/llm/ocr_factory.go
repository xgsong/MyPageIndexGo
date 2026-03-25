package llm

import (
	"fmt"

	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
)

type OCRClientFactory struct {
	cfg *config.Config
}

func NewOCRClientFactory(cfg *config.Config) *OCRClientFactory {
	return &OCRClientFactory{
		cfg: cfg,
	}
}

func (f *OCRClientFactory) CreateOCRClient() (document.OCRClient, error) {
	if !f.cfg.OCREnabled {
		return nil, fmt.Errorf("OCR is not enabled in configuration")
	}

	if f.cfg.OpenAIOCRBaseURL == "" {
		return nil, fmt.Errorf("openai_ocr_base_url is required when ocr_enabled is true")
	}
	if f.cfg.OCRModel == "" {
		return nil, fmt.Errorf("ocr_model is required when ocr_enabled is true")
	}

	return NewOpenAIOCRClient(f.cfg), nil
}
