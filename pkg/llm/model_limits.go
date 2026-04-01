package llm

// GetModelContextLimit returns the maximum context size (in tokens) for a given model.
// This is based on OpenAI's model specifications.
func GetModelContextLimit(model string) int {
	// Common OpenAI model context limits
	switch model {
	// GPT-4 models
	case "gpt-4", "gpt-4-0613":
		return 8192
	case "gpt-4-32k", "gpt-4-32k-0613":
		return 32768
	case "gpt-4-turbo", "gpt-4-turbo-2024-04-09":
		return 128000
	case "gpt-4o", "gpt-4o-2024-05-13":
		return 128000
	case "gpt-4o-mini", "gpt-4o-mini-2024-07-18":
		return 128000
	
	// GPT-3.5 models
	case "gpt-3.5-turbo", "gpt-3.5-turbo-0613":
		return 4096
	case "gpt-3.5-turbo-16k", "gpt-3.5-turbo-16k-0613":
		return 16384
	case "gpt-3.5-turbo-instruct":
		return 4096
	
	// Claude models (Anthropic)
	case "claude-3-opus-20240229":
		return 200000
	case "claude-3-sonnet-20240229":
		return 200000
	case "claude-3-haiku-20240307":
		return 200000
	case "claude-2.1":
		return 200000
	case "claude-2.0":
		return 100000
	case "claude-instant-1.2":
		return 100000
	
	// Local models (common sizes)
	case "Qwen2.5-7B-Instruct", "Qwen2.5-14B-Instruct", "Qwen2.5-32B-Instruct":
		return 32768
	case "Qwen2.5-72B-Instruct":
		return 32768
	case "Qwen2.5-Coder-7B-Instruct", "Qwen2.5-Coder-14B-Instruct":
		return 32768
	case "Qwen2.5-Math-7B-Instruct", "Qwen2.5-Math-14B-Instruct":
		return 32768
	case "Llama-3.2-1B-Instruct", "Llama-3.2-3B-Instruct":
		return 8192
	case "Llama-3.2-11B-Vision-Instruct":
		return 8192
	case "Llama-3.2-90B-Vision-Instruct":
		return 8192
	case "Mistral-7B-Instruct-v0.3":
		return 32768
	case "Mixtral-8x7B-Instruct-v0.1":
		return 32768
	case "Mixtral-8x22B-Instruct-v0.1":
		return 65536
	case "Gemma-2-2B-it", "Gemma-2-9B-it", "Gemma-2-27B-it":
		return 8192
	case "Phi-3-mini-4k-instruct", "Phi-3-small-8k-instruct", "Phi-3-medium-4k-instruct":
		return 4096
	
	// Default fallback for unknown models
	default:
		// Check if it's a Qwen model
		if len(model) >= 4 && model[:4] == "Qwen" {
			return 32768
		}
		// Check if it's a Llama model
		if len(model) >= 5 && model[:5] == "Llama" {
			return 8192
		}
		// Conservative default for unknown models
		return 32768
	}
}

// GetSafeBatchTokenLimit returns a safe token limit for batch processing.
// It reserves space for the prompt template and system messages.
func GetSafeBatchTokenLimit(model string) int {
	contextLimit := GetModelContextLimit(model)
	
	// Reserve tokens for:
	// 1. System message (~100 tokens)
	// 2. Batch prompt template (~500 tokens)
	// 3. JSON structure overhead (~200 tokens per request)
	// 4. Response tokens (~1000 tokens for all summaries)
	reservedTokens := 100 + 500 + 1000 // Base reserved tokens
	
	// Return 80% of context limit minus reserved tokens for safety
	safeLimit := (contextLimit * 80 / 100) - reservedTokens
	
	// Ensure minimum safe limit
	if safeLimit < 1000 {
		return 1000
	}
	
	return safeLimit
}