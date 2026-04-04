package readingtime

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "readingtime"
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

	return mins
}

func (p *Plugin) OnDocumentParsed(doc *content.Document) error {
	_ = doc
	return nil
}

func (p *Plugin) OnContext(ctx *renderer.ViewData) error {
	if ctx.Page == nil {
		return nil
	}

	if ctx.Data == nil {
		ctx.Data = map[string]any{}
	}

	words := countWords(ctx.Page.RawBody)
	ctx.Data["reading_time"] = estimateMinutes(words)
	ctx.Data["word_count"] = words

	return nil
}

func (p *Plugin) OnHTMLSlots(ctx *renderer.ViewData, slots *renderer.Slots) error {
	if ctx.Page == nil || ctx.Page.Type != "post" {
		return nil
	}

	wordCount := countWords(ctx.Page.RawBody)
	readingTime := estimateMinutes(wordCount)

	html := template.HTML(fmt.Sprintf(`
<div class="meta-panel-block">
  <h3>Reading</h3>
  <div class="meta-list">
    <div><strong>Reading time:</strong> %v min</div>
    <div><strong>Words:</strong> %v</div>
  </div>
</div>
`, readingTime, wordCount))

	slots.Add("post.sidebar.top", html)

	return nil
}

func init() {
	plugins.Register("readingtime", func() plugins.Plugin {
		return &Plugin{}
	})
}
