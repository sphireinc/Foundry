package toc

import (
	"html/template"
	"regexp"
	"strconv"
	"strings"

	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
)

type Plugin struct{}

type Item struct {
	Level int
	Text  string
	ID    string
}

var headingRE = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
var stripInlineCodeRE = regexp.MustCompile("`([^`]*)`")
var stripMarkdownLinkRE = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
var stripEmphasisRE = regexp.MustCompile(`[*_~]+`)
var invalidSlugCharsRE = regexp.MustCompile(`[^a-z0-9\s-]`)
var multiDashRE = regexp.MustCompile(`-+`)
var multiSpaceRE = regexp.MustCompile(`\s+`)

func (p *Plugin) Name() string {
	return "toc"
}

func (p *Plugin) OnDocumentParsed(doc *content.Document) error {
	items := extractTOC(doc.RawBody)

	if doc.Fields == nil {
		doc.Fields = map[string]any{}
	}

	doc.Fields["toc"] = items
	doc.Fields["has_toc"] = len(items) > 0

	return nil
}

func (p *Plugin) OnContext(ctx *renderer.ViewData) error {
	if ctx.Page == nil || ctx.Page.Fields == nil {
		return nil
	}

	if ctx.Data == nil {
		ctx.Data = map[string]any{}
	}

	if toc, ok := ctx.Page.Fields["toc"]; ok {
		ctx.Data["toc"] = toc
	}
	if hasTOC, ok := ctx.Page.Fields["has_toc"]; ok {
		ctx.Data["has_toc"] = hasTOC
	}

	return nil
}

func (p *Plugin) OnAssets(ctx *renderer.ViewData, assetSet *renderer.AssetSet) error {
	if ctx.Page == nil || ctx.Page.Type != "post" {
		return nil
	}

	assetSet.AddStyle("/plugins/toc/css/toc.css")
	return nil
}

func (p *Plugin) OnHTMLSlots(ctx *renderer.ViewData, slots *renderer.Slots) error {
	if ctx.Page == nil || ctx.Page.Type != "post" || ctx.Page.Fields == nil {
		return nil
	}

	raw, ok := ctx.Page.Fields["toc"]
	if !ok {
		return nil
	}

	items, ok := raw.([]Item)
	if !ok || len(items) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(`
<div class="meta-panel-block">
  <h3>On this page</h3>
  <div class="toc-list">
`)

	for _, item := range items {
		sb.WriteString(`<a class="toc-link toc-level-`)
		sb.WriteString(strconv.Itoa(item.Level))
		sb.WriteString(`" href="#`)
		sb.WriteString(template.HTMLEscapeString(item.ID))
		sb.WriteString(`">`)
		sb.WriteString(template.HTMLEscapeString(item.Text))
		sb.WriteString(`</a>`)
	}

	sb.WriteString(`
  </div>
</div>
`)

	slots.Add("post.sidebar.bottom", template.HTML(sb.String()))
	return nil
}

func extractTOC(body string) []Item {
	lines := strings.Split(body, "\n")
	items := make([]Item, 0)
	used := make(map[string]int)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := headingRE.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}

		level := len(matches[1])
		text := normalizeHeadingText(matches[2])
		if text == "" {
			continue
		}

		id := slugify(text)
		if id == "" {
			continue
		}

		if n, exists := used[id]; exists {
			n++
			used[id] = n
			id = id + "-" + strconv.Itoa(n)
		} else {
			used[id] = 0
		}

		items = append(items, Item{
			Level: level,
			Text:  text,
			ID:    id,
		})
	}

	return items
}

func normalizeHeadingText(s string) string {
	s = strings.TrimSpace(s)
	s = stripInlineCodeRE.ReplaceAllString(s, "$1")
	s = stripMarkdownLinkRE.ReplaceAllString(s, "$1")
	s = stripEmphasisRE.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	return s
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = invalidSlugCharsRE.ReplaceAllString(s, "")
	s = multiSpaceRE.ReplaceAllString(s, "-")
	s = multiDashRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func init() {
	plugins.Register("toc", func() plugins.Plugin {
		return &Plugin{}
	})
}
