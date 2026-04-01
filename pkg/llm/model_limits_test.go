package llm

import (
	"testing"
)

func TestGetModelContextLimit(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		// GPT-4 models
		{"gpt-4", 8192},
		{"gpt-4-32k", 32768},
		{"gpt-4-turbo", 128000},
		{"gpt-4o", 128000},
		{"gpt-4o-mini", 128000},
		
		// GPT-3.5 models
		{"gpt-3.5-turbo", 4096},
		{"gpt-3.5-turbo-16k", 16384},
		
		// Local models
		{"Qwen2.5-7B-Instruct", 32768},
		{"Qwen2.5-14B-Instruct", 32768},
		{"Llama-3.2-1B-Instruct", 8192},
		{"Mistral-7B-Instruct-v0.3", 32768},
		
		// Unknown models
		{"unknown-model", 32768}, // Default fallback
		{"Qwen-custom", 32768},   // Qwen prefix detection
		{"Llama-custom", 8192},   // Llama prefix detection
	}
	
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := GetModelContextLimit(tt.model)
			if result != tt.expected {
				t.Errorf("GetModelContextLimit(%q) = %d, want %d", tt.model, result, tt.expected)
			}
		})
	}
}

func TestGetSafeBatchTokenLimit(t *testing.T) {
	tests := []struct {
		model       string
		minExpected int
		maxExpected int
	}{
		{"gpt-4", 4900, 5000},           // 8192 * 0.8 - 1600 = 4953
		{"gpt-4-32k", 24500, 24700},     // 32768 * 0.8 - 1600 = 24614
		{"gpt-4-turbo", 100700, 100900}, // 128000 * 0.8 - 1600 = 100800
		{"Qwen2.5-7B-Instruct", 24500, 24700}, // 32768 * 0.8 - 1600 = 24614
	}
	
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := GetSafeBatchTokenLimit(tt.model)
			if result < tt.minExpected || result > tt.maxExpected {
				t.Errorf("GetSafeBatchTokenLimit(%q) = %d, want between %d and %d", 
					tt.model, result, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestGetSafeBatchTokenLimitMinimum(t *testing.T) {
	// Test that we always get at least 1000 tokens
	result := GetSafeBatchTokenLimit("tiny-model")
	if result < 1000 {
		t.Errorf("GetSafeBatchTokenLimit should return at least 1000, got %d", result)
	}
}