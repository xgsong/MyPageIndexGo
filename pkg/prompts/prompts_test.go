package prompts

import (
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderTemplate(t *testing.T) {
	t.Run("simple template", func(t *testing.T) {
		tmpl := `Hello, {{.Name}}!`
		data := TemplateData{"Name": "World"}
		result, err := RenderTemplateString(tmpl, data)
		require.NoError(t, err)
		assert.Equal(t, "Hello, World!", result)
	})

	t.Run("multiple variables", func(t *testing.T) {
		tmpl := `{{.Greeting}}, {{.Name}}!`
		data := TemplateData{"Greeting": "Hi", "Name": "Alice"}
		result, err := RenderTemplateString(tmpl, data)
		require.NoError(t, err)
		assert.Equal(t, "Hi, Alice!", result)
	})

	t.Run("invalid template syntax", func(t *testing.T) {
		tmpl := `{{.Invalid`
		_, err := RenderTemplateString(tmpl, TemplateData{})
		assert.Error(t, err)
	})

	t.Run("missing variable", func(t *testing.T) {
		tmpl := `Hello, {{.Missing}}!`
		result, err := RenderTemplateString(tmpl, TemplateData{})
		require.NoError(t, err)
		assert.Equal(t, "Hello, <no value>!", result)
	})

	t.Run("empty template", func(t *testing.T) {
		tmpl := ``
		result, err := RenderTemplateString(tmpl, TemplateData{})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("template with numbers", func(t *testing.T) {
		tmpl := `Count: {{.Count}}`
		data := TemplateData{"Count": 42}
		result, err := RenderTemplateString(tmpl, data)
		require.NoError(t, err)
		assert.Equal(t, "Count: 42", result)
	})
}

func TestRenderTemplateWithPrecompiled(t *testing.T) {
	t.Run("precompiled template", func(t *testing.T) {
		tmplStr := `Hello, {{.Name}}!`
		tmpl, err := template.New("test").Parse(tmplStr)
		require.NoError(t, err)

		result, err := RenderTemplate(tmpl, TemplateData{"Name": "Test"})
		require.NoError(t, err)
		assert.Equal(t, "Hello, Test!", result)
	})
}

func TestRenderSearchPrompt(t *testing.T) {
	t.Run("basic search prompt", func(t *testing.T) {
		query := "What is the revenue?"
		treeJSON := `{"title": "Financial Report"}`
		
		result, err := RenderSearchPrompt(query, treeJSON)
		require.NoError(t, err)
		assert.Contains(t, result, query)
		assert.Contains(t, result, treeJSON)
		assert.Contains(t, result, "<USER_QUERY_START>")
		assert.Contains(t, result, "<DOCUMENT_INDEX_START>")
	})

	t.Run("special characters in query", func(t *testing.T) {
		query := "What is 10% growth?"
		treeJSON := `{"key": "value"}`
		
		result, err := RenderSearchPrompt(query, treeJSON)
		require.NoError(t, err)
		assert.Contains(t, result, query)
	})

	t.Run("empty query", func(t *testing.T) {
		treeJSON := `{"title": "test"}`
		
		result, err := RenderSearchPrompt("", treeJSON)
		require.NoError(t, err)
		assert.Contains(t, result, treeJSON)
	})

	t.Run("multiline content", func(t *testing.T) {
		query := "Line 1\nLine 2"
		treeJSON := "{\n  \"title\": \"test\"\n}"
		
		result, err := RenderSearchPrompt(query, treeJSON)
		require.NoError(t, err)
		assert.Contains(t, result, query)
		assert.Contains(t, result, treeJSON)
	})
}

func TestGenerateStructurePrompt(t *testing.T) {
	t.Run("returns structure prompt", func(t *testing.T) {
		result := GenerateStructurePrompt()
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "document structure analyzer")
		assert.Contains(t, result, "hierarchical table of contents")
		assert.Contains(t, result, "JSON")
	})

	t.Run("prompt contains language instruction", func(t *testing.T) {
		result := GenerateStructurePrompt()
		assert.Contains(t, result, "CRITICAL LANGUAGE INSTRUCTION")
		assert.Contains(t, result, "EXACT SAME LANGUAGE")
	})

	t.Run("prompt contains format example", func(t *testing.T) {
		result := GenerateStructurePrompt()
		assert.Contains(t, result, "\"title\"")
		assert.Contains(t, result, "\"start_page\"")
		assert.Contains(t, result, "\"children\"")
	})
}

func TestRenderSummaryPrompt(t *testing.T) {
	t.Run("basic summary prompt", func(t *testing.T) {
		nodeTitle := "Revenue Section"
		text := "The company revenue increased by 15%."
		
		result, err := RenderSummaryPrompt(nodeTitle, text)
		require.NoError(t, err)
		assert.Contains(t, result, nodeTitle)
		assert.Contains(t, result, text)
		assert.Contains(t, result, "<NODE_TITLE_START>")
		assert.Contains(t, result, "<TEXT_CONTENT_START>")
	})

	t.Run("long text", func(t *testing.T) {
		nodeTitle := "Summary"
		text := strings.Repeat("This is a long text. ", 100)
		
		result, err := RenderSummaryPrompt(nodeTitle, text)
		require.NoError(t, err)
		assert.Contains(t, result, nodeTitle)
		assert.Contains(t, result, text[:50])
	})

	t.Run("special characters", func(t *testing.T) {
		nodeTitle := "Section: Overview"
		text := "Key points: 1) Growth 2) Innovation"
		
		result, err := RenderSummaryPrompt(nodeTitle, text)
		require.NoError(t, err)
		assert.Contains(t, result, nodeTitle)
		assert.Contains(t, result, text)
	})
}

func TestBatchSummaryPrompt(t *testing.T) {
	t.Run("returns batch summary prompt", func(t *testing.T) {
		result := BatchSummaryPrompt()
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "multiple sections")
		assert.Contains(t, result, "JSON array")
		assert.Contains(t, result, "node_id")
	})

	t.Run("prompt contains language instruction", func(t *testing.T) {
		result := BatchSummaryPrompt()
		assert.Contains(t, result, "CRITICAL LANGUAGE INSTRUCTION")
		assert.Contains(t, result, "EXACT SAME LANGUAGE")
	})

	t.Run("prompt contains example", func(t *testing.T) {
		result := BatchSummaryPrompt()
		assert.Contains(t, result, "Example output format")
		assert.Contains(t, result, "node_id")
		assert.Contains(t, result, "summary")
	})
}

func TestTOCDetectorPrompt(t *testing.T) {
	t.Run("basic TOC detector prompt", func(t *testing.T) {
		content := "Table of Contents\n1. Introduction\n2. Conclusion"
		result := TOCDetectorPrompt(content)
		assert.Contains(t, result, content)
		assert.Contains(t, result, "toc_detected")
		assert.Contains(t, result, "yes or no")
	})

	t.Run("empty content", func(t *testing.T) {
		result := TOCDetectorPrompt("")
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "given text")
	})
}

func TestTOCTransformerPrompt(t *testing.T) {
	t.Run("basic TOC transformer prompt", func(t *testing.T) {
		tocContent := "1. Introduction\n2. Conclusion"
		result := TOCTransformerPrompt(tocContent)
		assert.Contains(t, result, tocContent)
		assert.Contains(t, result, "table_of_contents")
		assert.Contains(t, result, "structure")
		assert.Contains(t, result, "page")
	})

	t.Run("JSON format instruction", func(t *testing.T) {
		result := TOCTransformerPrompt("content")
		assert.Contains(t, result, "JSON")
		assert.Contains(t, result, "table_of_contents")
	})
}

func TestTOCIndexExtractorPrompt(t *testing.T) {
	t.Run("basic TOC index extractor prompt", func(t *testing.T) {
		tocJSON := `{"table_of_contents": []}`
		content := "Page 1 content"
		result := TOCIndexExtractorPrompt(tocJSON, content)
		assert.Contains(t, result, tocJSON)
		assert.Contains(t, result, content)
		assert.Contains(t, result, "physical_index")
	})

	t.Run("contains format example", func(t *testing.T) {
		result := TOCIndexExtractorPrompt("{}", "content")
		assert.Contains(t, result, "<physical_index_X>")
		assert.Contains(t, result, "structure index")
	})
}

func TestTOCCompletenessCheckPrompt(t *testing.T) {
	t.Run("basic completeness check prompt", func(t *testing.T) {
		rawContent := "Original content"
		transformedContent := "Transformed content"
		result := TOCCompletenessCheckPrompt(rawContent, transformedContent)
		assert.Contains(t, result, rawContent)
		assert.Contains(t, result, transformedContent)
		assert.Contains(t, result, "completed")
		assert.Contains(t, result, "yes")
	})
}

func TestTOCContinuePrompt(t *testing.T) {
	t.Run("basic TOC continue prompt", func(t *testing.T) {
		rawContent := "Original TOC"
		incompleteContent := "Incomplete TOC"
		result := TOCContinuePrompt(rawContent, incompleteContent)
		assert.Contains(t, result, rawContent)
		assert.Contains(t, result, incompleteContent)
		assert.Contains(t, result, "continue")
		assert.Contains(t, result, "remaining part")
	})
}

func TestGetLanguageInstructionForTOC(t *testing.T) {
	tests := []struct {
		langCode string
		expected string
	}{
		{"zh", "Chinese"},
		{"ja", "Japanese"},
		{"ko", "Korean"},
		{"ru", "Russian"},
		{"fr", "French"},
		{"de", "German"},
		{"es", "Spanish"},
		{"en", ""},
		{"", ""},
		{"invalid", ""},
	}

	for _, tt := range tests {
		t.Run(tt.langCode, func(t *testing.T) {
			result := GetLanguageInstructionForTOC(tt.langCode)
			if tt.expected != "" {
				assert.Contains(t, result, tt.expected)
			} else {
				assert.Empty(t, result)
			}
		})
	}
}

func TestPromptConstants(t *testing.T) {
	t.Run("search template is defined", func(t *testing.T) {
		assert.NotNil(t, SearchTemplate)
		assert.NotEmpty(t, searchPromptTmpl)
	})

	t.Run("summary template is defined", func(t *testing.T) {
		assert.NotNil(t, SummaryTemplate)
		assert.NotEmpty(t, summaryPromptTmpl)
	})

	t.Run("structure prompt is defined", func(t *testing.T) {
		assert.NotEmpty(t, StructurePrompt)
	})
}
