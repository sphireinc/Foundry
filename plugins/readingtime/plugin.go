package readingtime

import (
	"fmt"
	"strings"

	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "reading-time"
}

func countWords(s string) int {
	return len(strings.Fields(s))
}

func estimateMinutes(words int) int {
	const wordsPerMinute = 200

	mins := words / wordsPerMinute
	if words%wordsPerMinute != 0 {
		mins++
	}

	if mins < 1 {
		mins = 1
	}

	fmt.Println(mins)

	return mins
}

func (p *Plugin) OnDocumentParsed(doc *content.Document) error {
	words := countWords(doc.RawBody)
	minutes := estimateMinutes(words)

	fmt.Println(words, minutes)

	if doc.Fields == nil {
		doc.Fields = map[string]any{}
	}

	doc.Fields["reading_time"] = minutes
	doc.Fields["word_count"] = words

	return nil
}

func (p *Plugin) OnContext(ctx *renderer.ViewData) error {
	if ctx.Page == nil {
		return nil
	}

	if ctx.Data == nil {
		ctx.Data = map[string]any{}
	}

	if ctx.Page.Fields != nil {
		ctx.Data["reading_time"] = ctx.Page.Fields["reading_time"]
		ctx.Data["word_count"] = ctx.Page.Fields["word_count"]
	}

	return nil
}

func init() {
	plugins.Register("reading-time", func() plugins.Plugin {
		return &Plugin{}
	})
}
