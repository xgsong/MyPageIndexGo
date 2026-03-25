package document

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/mypageindexgo/pkg/config"
)

func TestNewOCRClientFactory(t *testing.T) {
	cfg := config.DefaultConfig()
	factory := NewOCRClientFactory(cfg)
	assert.NotNil(t, factory)
	assert.Equal(t, cfg, factory.cfg)
}

func TestCreateOCRClient_OCRDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OCREnabled = false
	factory := NewOCRClientFactory(cfg)

	client, err := factory.CreateOCRClient()
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "OCR is not enabled")
}

func TestCreateOCRClient_UnsupportedType(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OCREnabled = true
	cfg.OCRType = "unsupported"
	factory := NewOCRClientFactory(cfg)

	client, err := factory.CreateOCRClient()
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "unsupported OCR type")
}

func TestCreateOCRClient_TesseractNotImplemented(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OCREnabled = true
	cfg.OCRType = "tesseract"
	factory := NewOCRClientFactory(cfg)

	client, err := factory.CreateOCRClient()
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestCreateOCRClient_CloudProvidersNotImplemented(t *testing.T) {
	providers := []string{"aliyun", "tencent", "baidu"}
	for _, p := range providers {
		cfg := config.DefaultConfig()
		cfg.OCREnabled = true
		cfg.OCRType = p
		factory := NewOCRClientFactory(cfg)

		client, err := factory.CreateOCRClient()
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "not yet implemented", "Provider: %s", p)
	}
}

func TestCreateOCRClient_LlamaCpp_MissingURL(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OCREnabled = true
	cfg.OCRType = "llama_cpp"
	cfg.LlamaCppServerURL = ""
	factory := NewOCRClientFactory(cfg)

	client, err := factory.CreateOCRClient()
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "llama_cpp_server_url is required")
}

func TestCreateOCRClient_LlamaCpp_MissingModel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OCREnabled = true
	cfg.OCRType = "llama_cpp"
	cfg.LlamaCppServerURL = "http://localhost:8080"
	cfg.OCRModel = ""
	factory := NewOCRClientFactory(cfg)

	client, err := factory.CreateOCRClient()
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "ocr_model is required")
}

func TestCreateOCRClient_LlamaCpp_Success(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OCREnabled = true
	cfg.OCRType = "llama_cpp"
	cfg.LlamaCppServerURL = "http://localhost:8080"
	cfg.OCRModel = "GLM-OCR-Q8_0"
	factory := NewOCRClientFactory(cfg)

	client, err := factory.CreateOCRClient()
	assert.NoError(t, err)
	assert.NotNil(t, client)
	_, ok := client.(*LlamaCppOCRClient)
	assert.True(t, ok)
}

func TestSupportedOCRTypes(t *testing.T) {
	types := SupportedOCRTypes()
	assert.Contains(t, types, "llama_cpp")
	assert.NotContains(t, types, "tesseract")
}