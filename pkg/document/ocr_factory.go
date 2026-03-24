package document

import (
	"fmt"
	"time"

	"github.com/xgsong/mypageindexgo/pkg/config"
)

// OCRClientFactory creates OCR clients based on configuration.
type OCRClientFactory struct {
	cfg *config.Config
}

// NewOCRClientFactory creates a new OCRClientFactory with the given configuration.
func NewOCRClientFactory(cfg *config.Config) *OCRClientFactory {
	return &OCRClientFactory{
		cfg: cfg,
	}
}

// CreateOCRClient creates an OCR client based on the configured OCR type.
func (f *OCRClientFactory) CreateOCRClient() (OCRClient, error) {
	if !f.cfg.OCREnabled {
		return nil, fmt.Errorf("OCR is not enabled in configuration")
	}

	switch f.cfg.OCRType {
	case "llama_cpp":
		return f.createLlamaCppClient()
	case "tesseract":
		return nil, fmt.Errorf("tesseract OCR is not yet implemented")
	case "aliyun", "tencent", "baidu":
		return nil, fmt.Errorf("cloud OCR providers are not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported OCR type: %s", f.cfg.OCRType)
	}
}

// createLlamaCppClient creates a LlamaCppOCRClient from configuration.
func (f *OCRClientFactory) createLlamaCppClient() (OCRClient, error) {
	if f.cfg.LlamaCppServerURL == "" {
		return nil, fmt.Errorf("llama_cpp_server_url is required for llama_cpp OCR type")
	}
	if f.cfg.OCRModel == "" {
		return nil, fmt.Errorf("ocr_model is required for llama_cpp OCR type")
	}

	config := LlamaCppOCRConfig{
		ServerURL: f.cfg.LlamaCppServerURL,
		ModelName: f.cfg.OCRModel,
		Timeout:   time.Duration(f.cfg.OCRTimeout) * time.Second,
	}

	return NewLlamaCppOCRClient(config)
}

// SupportedOCRTypes returns the list of supported OCR provider types.
func SupportedOCRTypes() []string {
	return []string{
		"llama_cpp",
		// "tesseract", // Future support
		// "aliyun",    // Future support
		// "tencent",   // Future support
		// "baidu",     // Future support
	}
}