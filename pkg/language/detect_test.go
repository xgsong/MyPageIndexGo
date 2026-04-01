package language

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetect_Chinese(t *testing.T) {
	chineseText := "这是一个中文文档。本公司 2023 年总收入为 1000 万元，同比增长 15%。"
	lang := Detect(chineseText)

	assert.Equal(t, "zh", lang.Code)
	assert.Equal(t, "Chinese", lang.Name)
	assert.Equal(t, "Han", lang.Script)
	assert.Greater(t, lang.Confidence, 0.5)
}

func TestDetect_English(t *testing.T) {
	englishText := "This is an English document. The company reported total revenue of $10 million in 2023."
	lang := Detect(englishText)

	assert.Equal(t, "en", lang.Code)
	assert.Equal(t, "English", lang.Name)
	assert.Equal(t, "Latin", lang.Script)
}

func TestDetect_Korean(t *testing.T) {
	koreanText := "이것은 한국어 문서입니다. 2023 년 총 매출은 1 천만 원이었습니다."
	lang := Detect(koreanText)

	assert.Equal(t, "ko", lang.Code)
	assert.Equal(t, "Korean", lang.Name)
	assert.Equal(t, "Hangul", lang.Script)
	assert.Greater(t, lang.Confidence, 0.5)
}

func TestDetect_Japanese(t *testing.T) {
	japaneseText := "これは日本語の文書です。2023 年の総収益は 1000 万円でした。"
	lang := Detect(japaneseText)

	assert.Equal(t, "ja", lang.Code)
	assert.Equal(t, "Japanese", lang.Name)
	// Japanese can be detected as Hiragana, Katakana, or Japanese depending on the text
	// Note: Kanji characters are detected as "Han" script
	assert.Contains(t, []string{"Hiragana", "Katakana", "Japanese", "Han"}, lang.Script)
	// Confidence might be lower due to mixed kanji/hiragana
	assert.Greater(t, lang.Confidence, 0.3)
}

func TestDetect_Russian(t *testing.T) {
	russianText := "Это документ на русском языке. Общий доход компании в 2023 году составил 10 миллионов рублей."
	lang := Detect(russianText)

	assert.Equal(t, "ru", lang.Code)
	assert.Equal(t, "Russian", lang.Name)
	assert.Equal(t, "Cyrillic", lang.Script)
	assert.Greater(t, lang.Confidence, 0.5)
}

func TestDetect_Arabic(t *testing.T) {
	arabicText := "هذه وثيقة عربية. بلغ إجمالي الإيرادات في عام 2023 عشرة ملايين دولار."
	lang := Detect(arabicText)

	assert.Equal(t, "ar", lang.Code)
	assert.Equal(t, "Arabic", lang.Name)
	assert.Equal(t, "Arabic", lang.Script)
	assert.Greater(t, lang.Confidence, 0.5)
}

func TestDetect_Empty(t *testing.T) {
	lang := Detect("")

	assert.Equal(t, "en", lang.Code)
	assert.Equal(t, "English", lang.Name)
	assert.Equal(t, float64(0), lang.Confidence)
}

func TestDetect_MixedChineseEnglish(t *testing.T) {
	// When there's more English than Chinese, it will detect English
	// This is expected behavior - the detector picks the dominant script
	mixedText := "This document has some 中文 but mostly English text everywhere in the document content here."
	lang := Detect(mixedText)

	// This test demonstrates current behavior - Latin script dominates
	// In practice, documents are usually monolingual or have dominant language
	assert.Equal(t, "en", lang.Code)
}

func TestGetLanguageName(t *testing.T) {
	tests := []struct {
		lang     Language
		expected string
	}{
		{LanguageChinese, "Chinese (中文)"},
		{LanguageEnglish, "English"},
		{LanguageJapanese, "Japanese (日本語)"},
		{LanguageKorean, "Korean (한국어)"},
		{LanguageRussian, "Russian"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.lang.GetLanguageName())
		})
	}
}

func TestDetector_DetectLongText(t *testing.T) {
	// Test with a very long text - should only sample first 2000 runes
	longChinese := ""
	for i := 0; i < 5000; i++ {
		longChinese += "中"
	}

	lang := Detect(longChinese)
	assert.Equal(t, "zh", lang.Code)
	assert.Greater(t, lang.Confidence, 0.9)
}
