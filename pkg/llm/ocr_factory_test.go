package llm

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

func TestCreateOCRClient_MissingURL(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OCREnabled = true
	cfg.OpenAIOCRBaseURL = ""
	factory := NewOCRClientFactory(cfg)

	client, err := factory.CreateOCRClient()
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "openai_ocr_base_url is required")
}

func TestCreateOCRClient_MissingModel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OCREnabled = true
	cfg.OpenAIOCRBaseURL = "http://localhost:8080"
	cfg.OCRModel = ""
	factory := NewOCRClientFactory(cfg)

	client, err := factory.CreateOCRClient()
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "ocr_model is required")
}

func TestCreateOCRClient_Success(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.OCREnabled = true
	cfg.OpenAIOCRBaseURL = "http://localhost:8080"
	cfg.OCRModel = "GLM-OCR-Q8_0"
	factory := NewOCRClientFactory(cfg)

	client, err := factory.CreateOCRClient()
	assert.NoError(t, err)
	assert.NotNil(t, client)
	_, ok := client.(*OpenAIOCRClient)
	assert.True(t, ok)
}
