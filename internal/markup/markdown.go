package markup

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
)

func MarkdownToHTML(input string) (template.HTML, error) {
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(input), &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
