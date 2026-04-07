package prompts

import (
	"bytes"
	"text/template"
)

type TemplateData map[string]any

func RenderTemplate(tmpl *template.Template, data TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func RenderTemplateString(tmplStr string, data TemplateData) (string, error) {
	tmpl, err := template.New("prompt").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	return RenderTemplate(tmpl, data)
}
