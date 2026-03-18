package markup

import (
	"bytes"
	"html/template"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	rendererhtml "github.com/yuin/goldmark/renderer/html"
)

var headingTagRE = regexp.MustCompile(`<(h[1-6])>(.*?)</h[1-6]>`)
var stripTagsRE = regexp.MustCompile(`<[^>]+>`)
var invalidSlugCharsRE = regexp.MustCompile(`[^a-z0-9\s-]`)
var multiDashRE = regexp.MustCompile(`-+`)
var multiSpaceRE = regexp.MustCompile(`\s+`)

func MarkdownToHTML(input string, allowUnsafeHTML bool) (template.HTML, error) {
	var buf bytes.Buffer

	md := goldmark.New()
	if allowUnsafeHTML {
		md = goldmark.New(goldmark.WithRendererOptions(rendererhtml.WithUnsafe()))
	}

	if err := md.Convert([]byte(input), &buf); err != nil {
		return "", err
	}

	html := addHeadingIDs(buf.String())
	return template.HTML(html), nil
}

func addHeadingIDs(html string) string {
	used := make(map[string]int)

	return headingTagRE.ReplaceAllStringFunc(html, func(match string) string {
		parts := headingTagRE.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		tag := parts[1]
		inner := parts[2]
		text := strings.TrimSpace(stripTagsRE.ReplaceAllString(inner, ""))
		if text == "" {
			return match
		}

		id := slugify(text)
		if id == "" {
			return match
		}

		if n, exists := used[id]; exists {
			n++
			used[id] = n
			id = id + "-" + strconv.Itoa(n)
		} else {
			used[id] = 0
		}

		return "<" + tag + ` id="` + id + `">` + inner + "</" + tag + ">"
	})
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = invalidSlugCharsRE.ReplaceAllString(s, "")
	s = multiSpaceRE.ReplaceAllString(s, "-")
	s = multiDashRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
