package syntaxhighlight

import (
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/renderer"
)

func TestPluginName(t *testing.T) {
	p := &Plugin{}
	if p.Name() != "syntaxhighlight" {
		t.Fatalf("unexpected plugin name: %q", p.Name())
	}
}

func TestOnAfterRenderNoCodeBlock(t *testing.T) {
	p := &Plugin{}
	input := []byte(`<p>Hello world</p>`)
	got, err := p.OnAfterRender("", input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(input) {
		t.Fatalf("expected input unchanged when no code block present")
	}
}

func TestOnAfterRenderWrapsBlock(t *testing.T) {
	p := &Plugin{}
	input := []byte(`<pre><code class="language-go">package main</code></pre>`)
	got, err := p.OnAfterRender("", input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, `class="sh-wrapper"`) {
		t.Fatal("expected sh-wrapper in output")
	}
	if !strings.Contains(out, `class="sh-toolbar"`) {
		t.Fatal("expected sh-toolbar when language is present")
	}
	if !strings.Contains(out, `data-lang="go"`) {
		t.Fatal("expected data-lang attribute on pre element")
	}
	if !strings.Contains(out, `sh-copy`) {
		t.Fatal("expected copy button in output")
	}
}

func TestOnAfterRenderNoLangNoToolbar(t *testing.T) {
	p := &Plugin{}
	input := []byte(`<pre><code>plain block</code></pre>`)
	got, err := p.OnAfterRender("", input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, `class="sh-wrapper"`) {
		t.Fatal("expected sh-wrapper in output")
	}
	if strings.Contains(out, `class="sh-toolbar"`) {
		t.Fatal("expected no toolbar when no language is detected")
	}
}

func TestOnAfterRenderMultipleBlocks(t *testing.T) {
	p := &Plugin{}
	input := []byte(`<pre><code class="language-go">a</code></pre><pre><code class="language-py">b</code></pre>`)
	got, err := p.OnAfterRender("", input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Count(string(got), "sh-wrapper") != 2 {
		t.Fatal("expected two wrapped blocks")
	}
}

func TestOnAssetsInjectsWhenCodePresent(t *testing.T) {
	p := &Plugin{}
	doc := &content.Document{
		Type:     "post",
		HTMLBody: `<pre><code class="language-go">x</code></pre>`,
	}
	ctx := &renderer.ViewData{Page: doc}
	assets := renderer.NewAssetSet()
	if err := p.OnAssets(ctx, assets); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	slots := renderer.NewSlots()
	assets.RenderInto(slots)
	head := string(slots.Render("head.end"))
	if !strings.Contains(head, "syntax-highlight.css") {
		t.Fatal("expected stylesheet injected into head.end")
	}
	body := string(slots.Render("body.end"))
	if !strings.Contains(body, "copy.js") {
		t.Fatal("expected copy.js injected into body.end")
	}
}

func TestOnAssetsSkipsWhenNoCode(t *testing.T) {
	p := &Plugin{}
	doc := &content.Document{Type: "post", HTMLBody: `<p>no code here</p>`}
	ctx := &renderer.ViewData{Page: doc}
	assets := renderer.NewAssetSet()
	if err := p.OnAssets(ctx, assets); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	slots := renderer.NewSlots()
	assets.RenderInto(slots)
	if strings.Contains(string(slots.Render("head.end")), "syntax-highlight") {
		t.Fatal("expected no stylesheet injected when page has no code blocks")
	}
}

func TestOnAssetsNilPage(t *testing.T) {
	p := &Plugin{}
	ctx := &renderer.ViewData{Page: nil}
	assets := renderer.NewAssetSet()
	if err := p.OnAssets(ctx, assets); err != nil {
		t.Fatalf("unexpected error on nil page: %v", err)
	}
}

func TestPageHasCodeBlock(t *testing.T) {
	if pageHasCodeBlock(`<pre><code>x</code></pre>`) != true {
		t.Fatal("expected true for html with code block")
	}
	if pageHasCodeBlock(`<p>no code</p>`) != false {
		t.Fatal("expected false for html without code block")
	}
}

func TestHighlightGo(t *testing.T) {
	out := highlightGo("func main() {}")
	if !strings.Contains(out, `sh-keyword`) {
		t.Fatal("expected keyword spans in Go output")
	}
}

func TestHighlightJS(t *testing.T) {
	out := highlightJS("const x = 1;")
	if !strings.Contains(out, `sh-keyword`) {
		t.Fatal("expected keyword spans in JS output")
	}
	if !strings.Contains(out, `sh-number`) {
		t.Fatal("expected number spans in JS output")
	}
}

func TestHighlightPython(t *testing.T) {
	out := highlightPython("def foo(): pass")
	if !strings.Contains(out, `sh-keyword`) {
		t.Fatal("expected keyword spans in Python output")
	}
}

func TestHighlightShell(t *testing.T) {
	out := highlightShell("# comment\nfoundry serve --debug")
	if !strings.Contains(out, `sh-comment`) {
		t.Fatal("expected comment spans in shell output")
	}
	if !strings.Contains(out, `sh-flag`) {
		t.Fatal("expected flag spans in shell output")
	}
}

func TestHighlightYAML(t *testing.T) {
	out := highlightYAML("title: Hello\n# comment")
	if !strings.Contains(out, `sh-keyword`) {
		t.Fatal("expected keyword spans in YAML output")
	}
	if !strings.Contains(out, `sh-comment`) {
		t.Fatal("expected comment spans in YAML output")
	}
}

func TestHighlightJSON(t *testing.T) {
	out := highlightJSON(`{"key": true, "n": 42}`)
	if !strings.Contains(out, `sh-keyword`) {
		t.Fatal("expected keyword spans in JSON output")
	}
	if !strings.Contains(out, `sh-number`) {
		t.Fatal("expected number spans in JSON output")
	}
}

func TestHighlightUnknownLangPassthrough(t *testing.T) {
	code := "some unknown language code"
	if highlightCode(code, "brainfuck") != code {
		t.Fatal("expected unknown language to pass through unchanged")
	}
}

func TestBuildKeywordRE(t *testing.T) {
	re := buildKeywordRE([]string{"for", "if"})
	if !re.MatchString("for") || !re.MatchString("if") {
		t.Fatal("expected keyword regexp to match listed keywords")
	}
	// must not match partial words
	if re.MatchString("format") {
		t.Fatal("expected keyword regexp not to match partial word 'format'")
	}
}
