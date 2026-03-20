package llm

import (
	"testing"

	"github.com/xgsong/mypageindexgo/pkg/config"
)

// This file just contains compile-time interface compliance checks.
// No actual tests are needed here - the functionality is tested in openai_test.go.

var _ LLMClient = (*OpenAIClient)(nil)

func TestInterfaceCompliance(t *testing.T) {
	// This test is just to ensure the compile-time check above is run.
	// If this compiles, the check passes.
	cfg := config.DefaultConfig()
	client := NewOpenAIClient(cfg)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}
