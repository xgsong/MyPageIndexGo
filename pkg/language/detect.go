package language

import (
	"unicode"
	"unicode/utf8"
)

// Language represents a detected document language.
type Language struct {
	// Code is the ISO 639-1 language code (e.g., "zh", "en", "fr").
	Code string `json:"code"`
	// Name is the human-readable language name (e.g., "Chinese", "English").
	Name string `json:"name"`
	// Script is the writing system (e.g., "Han", "Latin", "Cyrillic").
	Script string `json:"script"`
	// Confidence is the detection confidence (0.0 to 1.0).
	Confidence float64 `json:"confidence"`
}

// Common language constants.
var (
	LanguageChinese  = Language{Code: "zh", Name: "Chinese", Script: "Han", Confidence: 1.0}
	LanguageEnglish  = Language{Code: "en", Name: "English", Script: "Latin", Confidence: 1.0}
	LanguageKorean   = Language{Code: "ko", Name: "Korean", Script: "Hangul", Confidence: 1.0}
	LanguageJapanese = Language{Code: "ja", Name: "Japanese", Script: "Japanese", Confidence: 1.0}
	LanguageRussian  = Language{Code: "ru", Name: "Russian", Script: "Cyrillic", Confidence: 1.0}
	LanguageArabic   = Language{Code: "ar", Name: "Arabic", Script: "Arabic", Confidence: 1.0}
	LanguageFrench   = Language{Code: "fr", Name: "French", Script: "Latin", Confidence: 1.0}
	LanguageGerman   = Language{Code: "de", Name: "German", Script: "Latin", Confidence: 1.0}
	LanguageSpanish  = Language{Code: "es", Name: "Spanish", Script: "Latin", Confidence: 1.0}
	LanguageUnknown  = Language{Code: "en", Name: "English", Script: "Latin", Confidence: 0.0} // Default to English
)

// Detector detects the language of text using Unicode range analysis.
type Detector struct{}

// NewDetector creates a new language detector.
func NewDetector() *Detector {
	return &Detector{}
}

// Detect detects the language from text using Unicode character analysis.
// It analyzes the first 2000 characters for efficiency.
func (d *Detector) Detect(text string) Language {
	if len(text) == 0 {
		return LanguageUnknown
	}

	// Take sample of first 2000 runes for efficiency
	sample := text
	if utf8.RuneCountInString(sample) > 2000 {
		// Truncate to 2000 runes
		runes := []rune(sample)
		if len(runes) > 2000 {
			sample = string(runes[:2000])
		}
	}

	// Count characters by script
	counts := make(map[string]int)
	totalChars := 0

	for _, r := range sample {
		// Skip whitespace and punctuation
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}

		totalChars++
		script := getScript(r)
		if script != "" {
			counts[script]++
		}
	}

	if totalChars == 0 {
		return LanguageUnknown
	}

	// Find dominant script
	dominantScript := ""
	maxCount := 0
	for script, count := range counts {
		if count > maxCount {
			maxCount = count
			dominantScript = script
		}
	}

	// Calculate confidence
	confidence := float64(maxCount) / float64(totalChars)

	// Map script to language
	// Note: This is simplified - in practice, we'd need more context to distinguish
	// languages that share the same script (e.g., English vs French vs German)
	return getLanguageFromScript(dominantScript, confidence)
}

// getScript returns the script name for a rune.
func getScript(r rune) string {
	switch {
	// CJK Unified Ideographs (Chinese)
	case r >= 0x4E00 && r <= 0x9FFF:
		return "Han"
	// CJK Radicals Supplement
	case r >= 0x2E80 && r <= 0x2EFF:
		return "Han"
	// CJK Compatibility Ideographs
	case r >= 0xF900 && r <= 0xFAFF:
		return "Han"
	// CJK Unified Ideographs Extension A
	case r >= 0x3400 && r <= 0x4DBF:
		return "Han"

	// Hangul (Korean)
	case r >= 0xAC00 && r <= 0xD7A3: // Hangul Syllables
		return "Hangul"
	case r >= 0x1100 && r <= 0x11FF: // Hangul Jamo
		return "Hangul"
	case r >= 0x3130 && r <= 0x318F: // Hangul Compatibility Jamo
		return "Hangul"

	// Japanese Hiragana
	case r >= 0x3040 && r <= 0x309F:
		return "Hiragana"
	// Japanese Katakana
	case r >= 0x30A0 && r <= 0x30FF:
		return "Katakana"

	// Cyrillic (Russian, Ukrainian, etc.)
	case r >= 0x0400 && r <= 0x04FF:
		return "Cyrillic"
	case r >= 0x0500 && r <= 0x052F:
		return "Cyrillic"

	// Arabic
	case r >= 0x0600 && r <= 0x06FF:
		return "Arabic"
	case r >= 0x0750 && r <= 0x077F:
		return "Arabic"
	case r >= 0x08A0 && r <= 0x08FF:
		return "Arabic"

	// Devanagari (Hindi, Sanskrit)
	case r >= 0x0900 && r <= 0x097F:
		return "Devanagari"

	// Thai
	case r >= 0x0E00 && r <= 0x0E7F:
		return "Thai"

	// Latin script (English, French, German, Spanish, etc.)
	case r >= 0x0041 && r <= 0x007A: // Basic Latin
		return "Latin"
	case r >= 0x0080 && r <= 0x00FF: // Latin-1 Supplement
		return "Latin"
	case r >= 0x0100 && r <= 0x017F: // Latin Extended-A
		return "Latin"
	case r >= 0x0180 && r <= 0x024F: // Latin Extended-B
		return "Latin"

	default:
		return ""
	}
}

// getLanguageFromScript maps a script to a language.
func getLanguageFromScript(script string, confidence float64) Language {
	switch script {
	case "Han":
		return Language{Code: "zh", Name: "Chinese", Script: "Han", Confidence: confidence}
	case "Hangul":
		return Language{Code: "ko", Name: "Korean", Script: "Hangul", Confidence: confidence}
	case "Hiragana", "Katakana":
		return Language{Code: "ja", Name: "Japanese", Script: "Japanese", Confidence: confidence}
	case "Cyrillic":
		return Language{Code: "ru", Name: "Russian", Script: "Cyrillic", Confidence: confidence}
	case "Arabic":
		return Language{Code: "ar", Name: "Arabic", Script: "Arabic", Confidence: confidence}
	case "Devanagari":
		return Language{Code: "hi", Name: "Hindi", Script: "Devanagari", Confidence: confidence}
	case "Thai":
		return Language{Code: "th", Name: "Thai", Script: "Thai", Confidence: confidence}
	case "Latin":
		// Default to English for Latin script
		// In practice, we could add statistical analysis to distinguish
		// between English, French, German, Spanish, etc.
		return Language{Code: "en", Name: "English", Script: "Latin", Confidence: confidence * 0.8} // Lower confidence
	default:
		return LanguageUnknown
	}
}

// Detect is a convenience function to detect language from text.
func Detect(text string) Language {
	detector := NewDetector()
	return detector.Detect(text)
}

// GetLanguageName returns a formatted language name for display.
func (l Language) GetLanguageName() string {
	if l.Code == "zh" {
		return "Chinese (中文)"
	}
	if l.Code == "en" {
		return "English"
	}
	if l.Code == "ja" {
		return "Japanese (日本語)"
	}
	if l.Code == "ko" {
		return "Korean (한국어)"
	}
	return l.Name
}
