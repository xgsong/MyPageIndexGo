package llm

import (
	"bytes"
	"encoding/json"
	"text/template"

	"github.com/xgsong/mypageindexgo/pkg/document"
)

// GenerateStructurePrompt returns the fixed prompt for generating hierarchical table of contents.
func GenerateStructurePrompt() string {
	return `You are a document structure analyzer. Your task is to convert the given raw text from document pages into a hierarchical table of contents structure.

The input is raw text extracted from a range of pages in a document. Output a JSON object representing the hierarchical structure.

Your output must follow this exact JSON format:

{
  "title": "Main title of this section",
  "start_page": 1,
  "end_page": 5,
  "children": [
    {
      "title": "Subsection title",
      "start_page": 1,
      "end_page": 2,
      "children": []
    }
  ]
}

Important rules:
- "title" must ALWAYS be a string enclosed in double quotes
- "start_page" and "end_page" must ALWAYS be numbers (no quotes)
- Escape any double quotes or newlines inside the title string
- "children" must ALWAYS be an array (empty array if no children)
- Output ONLY the JSON object - no explanations, no extra text

Guidelines:
1. Identify the natural semantic sections and subsections based on headings and content breaks.
2. Preserve the original page numbering - use the actual page numbers from the text.
3. Create a hierarchical tree that reflects the document's organization.
4. Use descriptive, accurate titles that match exactly what's in the document.
5. If the text starts mid-section, do your best to infer the structure.

Input text:
`
}

// summaryPromptData is the template data for the summary prompt.
type summaryPromptData struct {
	NodeTitle string
	Text      string
}

var summaryPromptTemplate = template.Must(template.New("summary").Parse(`You are a document content summarizer. Given a section of text from a document and this node's title, create a concise key-point summary that captures ONLY the content relevant to this node's title.

Node title: {{.NodeTitle}}

Text content:
{{.Text}}

Guidelines:
1. ONLY summarize content that is directly relevant to the node title "{{.NodeTitle}}". Ignore content that belongs to other sections.
2. Focus ONLY on the key points, main arguments, and critical data for this specific section.
3. Remove verbose descriptions, examples, and unrelated content. Keep it extremely brief.
4. Preserve specific facts, numbers, names, and technical terms accurately.
5. Use the same language as the original text.
6. Don't add any introductory phrases or explanations - just output the summary text directly.
7. Maximum: 1-2 sentences, 50 words for short sections, 100 words maximum for any section.

Summary:
`))

// RenderSummaryPrompt renders the summary prompt with the node title and text.
func RenderSummaryPrompt(nodeTitle, text string) (string, error) {
	data := summaryPromptData{
		NodeTitle: nodeTitle,
		Text:      text,
	}

	var buf bytes.Buffer
	if err := summaryPromptTemplate.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// searchPromptTemplate is the Go template for the search prompt.
var searchPromptTemplate = template.Must(template.New("search").Parse(`You are a document search and reasoning engine. Given a user query and a hierarchical index of a document, find the most relevant sections that answer the query and provide a comprehensive answer.

The document index is:
{{.TreeJSON}}

User query: {{.Query}}

Instructions:
1. Analyze the query and understand what information is being requested.
2. Identify which nodes in the index tree are most relevant to answering the query.
3. Output a JSON object with your answer and the list of relevant node IDs:

{
  "answer": "Your detailed answer to the query based on the relevant sections",
  "node_ids": ["id1", "id2", ...]
}

Guidelines:
- If the answer isn't found in the document, say so clearly in the answer.
- Only include nodes that are actually relevant to the query.
- Base your answer only on the document content, not external knowledge.
- Provide a complete, detailed answer that addresses all aspects of the query.
`))

// SearchPrompt renders the search prompt with the given query and index tree.
func SearchPrompt(query string, tree *document.IndexTree) (string, error) {
	// Marshal the entire tree to JSON for the prompt
	treeJSON, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return "", err
	}

	templateData := struct {
		Query    string
		TreeJSON string
	}{
		Query:    query,
		TreeJSON: string(treeJSON),
	}

	var buf bytes.Buffer
	err = searchPromptTemplate.Execute(&buf, templateData)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
