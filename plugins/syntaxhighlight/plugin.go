package syntaxhighlight

import (
	"html/template"
	"regexp"
	"strings"

	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
)

var codeBlockRE = regexp.MustCompile(`(?s)<pre><code((?:\s+class="[^"]*")?)\s*>(.*?)</code></pre>`)

var langClassRE = regexp.MustCompile(`language-([a-zA-Z0-9_+\-]+)`)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "syntaxhighlight"
}

func (p *Plugin) OnAssets(ctx *renderer.ViewData, assets *renderer.AssetSet) error {
	if ctx.Page == nil {
		return nil
	}
	if !pageHasCodeBlock(string(ctx.Page.HTMLBody)) {
		return nil
	}
	assets.AddStyle("/plugins/syntaxhighlight/css/syntax-highlight.css")
	assets.AddScript("/plugins/syntaxhighlight/js/copy.js", renderer.ScriptPositionBodyEnd)
	return nil
}

func (p *Plugin) OnAfterRender(_ string, html []byte) ([]byte, error) {
	result := codeBlockRE.ReplaceAllFunc(html, func(match []byte) []byte {
		parts := codeBlockRE.FindSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		classAttr := string(parts[1])
		code := string(parts[2])

		lang := ""
		if m := langClassRE.FindStringSubmatch(classAttr); len(m) == 2 {
			lang = strings.ToLower(m[1])
		}

		highlighted := highlightCode(code, lang)

		var sb strings.Builder
		sb.WriteString(`<div class="sh-wrapper">`)
		if lang != "" {
			sb.WriteString(`<div class="sh-toolbar"><span class="sh-lang">`)
			sb.WriteString(template.HTMLEscapeString(lang))
			sb.WriteString(`</span>`)
			sb.WriteString(`<button class="sh-copy" onclick="foundryShCopy(this)" aria-label="Copy code">Copy</button>`)
			sb.WriteString(`</div>`)
		}
		sb.WriteString(`<pre class="sh-pre"`)
		if lang != "" {
			sb.WriteString(` data-lang="`)
			sb.WriteString(template.HTMLEscapeString(lang))
			sb.WriteString(`"`)
		}
		sb.WriteString(`><code`)
		if classAttr != "" {
			sb.WriteString(classAttr)
		}
		sb.WriteString(`>`)
		sb.WriteString(highlighted)
		sb.WriteString(`</code></pre>`)
		sb.WriteString(`</div>`)
		return []byte(sb.String())
	})
	return result, nil
}

func pageHasCodeBlock(html string) bool {
	return strings.Contains(html, "<pre><code")
}

func highlightCode(code, lang string) string {
	switch lang {
	case "go":
		return highlightGo(code)
	case "js", "javascript", "ts", "typescript":
		return highlightJS(code)
	case "py", "python":
		return highlightPython(code)
	case "sh", "bash", "shell", "zsh":
		return highlightShell(code)
	case "yaml", "yml":
		return highlightYAML(code)
	case "json":
		return highlightJSON(code)
	case "html":
		return highlightHTML(code)
	case "css":
		return highlightCSS(code)
	default:
		return code
	}
}

var (
	goKeywordRE = buildKeywordRE(goKeywords)
	goBuiltinRE = buildKeywordRE(goBuiltins)
	goCommentRE = regexp.MustCompile(`(//[^\n]*)`)
	goStringRE  = regexp.MustCompile("(?s)(`[^`]*`|&quot;(?:[^&]|&[^q]|&q[^u]|&qu[^o]|&quo[^t])*&quot;)")
	goNumberRE  = regexp.MustCompile(`\b(0x[0-9a-fA-F]+|\d+(?:\.\d+)?)\b`)

	jsKeywordRE = buildKeywordRE(jsKeywords)
	jsCommentRE = regexp.MustCompile(`(//[^\n]*|/\*(?s:.*?)\*/)`)
	jsStringRE  = regexp.MustCompile("(?s)(&quot;(?:[^&]|&[^q])*&quot;|&#39;[^&#]*&#39;)")
	jsNumberRE  = regexp.MustCompile(`\b(\d+(?:\.\d+)?)\b`)

	pyKeywordRE = buildKeywordRE(pyKeywords)
	pyCommentRE = regexp.MustCompile(`(#[^\n]*)`)
	pyStringRE  = regexp.MustCompile("(?s)(&quot;(?:[^&]|&[^q])*&quot;|&#39;[^&#]*&#39;)")
	pyNumberRE  = regexp.MustCompile(`\b(\d+(?:\.\d+)?)\b`)

	shCommentRE = regexp.MustCompile(`(#[^\n]*)`)
	shFlagRE    = regexp.MustCompile(`(--?[a-zA-Z][a-zA-Z0-9\-]*)`)

	yamlKeyRE     = regexp.MustCompile(`(?m)^(\s*)([a-zA-Z_][a-zA-Z0-9_\-]*)\s*(:)`)
	yamlValueRE   = regexp.MustCompile(`(?m)(:\s+)(.+)$`)
	yamlCommentRE = regexp.MustCompile(`(#[^\n]*)`)

	jsonStringRE = regexp.MustCompile(`(?s)(&quot;(?:[^&]|&[^q]|&q[^u]|&qu[^o]|&quo[^t])*&quot;)`)
	jsonNumberRE = regexp.MustCompile(`\b(\d+(?:\.\d+)?)\b`)
	jsonBoolRE   = regexp.MustCompile(`\b(true|false|null)\b`)

	htmlTagRE  = regexp.MustCompile(`(&lt;/?[a-zA-Z][a-zA-Z0-9\-]*(?:\s[^&gt;]*)??/?&gt;)`)
	htmlAttrRE = regexp.MustCompile(`\b([a-zA-Z\-]+)(=)`)

	cssPropRE     = regexp.MustCompile(`(?m)^\s*([a-zA-Z\-]+)\s*(:)`)
	cssValueRE    = regexp.MustCompile(`(:\s*)([^;{}\n]+)`)
	cssSelectorRE = regexp.MustCompile(`(?m)^([^\s{][^{]*)(\{)`)
	cssCommentRE  = regexp.MustCompile(`(/\*(?s:.*?)\*/)`)
)

func highlightGo(code string) string {
	code = goStringRE.ReplaceAllString(code, `<span class="sh-string">$0</span>`)
	code = goCommentRE.ReplaceAllString(code, `<span class="sh-comment">$1</span>`)
	code = goNumberRE.ReplaceAllString(code, `<span class="sh-number">$1</span>`)
	code = goBuiltinRE.ReplaceAllString(code, `<span class="sh-builtin">$1</span>`)
	code = goKeywordRE.ReplaceAllString(code, `<span class="sh-keyword">$1</span>`)
	return code
}

func highlightJS(code string) string {
	code = jsStringRE.ReplaceAllString(code, `<span class="sh-string">$0</span>`)
	code = jsCommentRE.ReplaceAllString(code, `<span class="sh-comment">$1</span>`)
	code = jsNumberRE.ReplaceAllString(code, `<span class="sh-number">$1</span>`)
	code = jsKeywordRE.ReplaceAllString(code, `<span class="sh-keyword">$1</span>`)
	return code
}

func highlightPython(code string) string {
	code = pyStringRE.ReplaceAllString(code, `<span class="sh-string">$0</span>`)
	code = pyCommentRE.ReplaceAllString(code, `<span class="sh-comment">$1</span>`)
	code = pyNumberRE.ReplaceAllString(code, `<span class="sh-number">$1</span>`)
	code = pyKeywordRE.ReplaceAllString(code, `<span class="sh-keyword">$1</span>`)
	return code
}

func highlightShell(code string) string {
	code = shFlagRE.ReplaceAllString(code, `<span class="sh-flag">$1</span>`)
	code = shCommentRE.ReplaceAllString(code, `<span class="sh-comment">$1</span>`)
	return code
}

func highlightYAML(code string) string {
	code = yamlCommentRE.ReplaceAllString(code, `<span class="sh-comment">$1</span>`)
	code = yamlKeyRE.ReplaceAllString(code, `$1<span class="sh-keyword">$2</span><span class="sh-punctuation">$3</span>`)
	code = yamlValueRE.ReplaceAllString(code, `$1<span class="sh-string">$2</span>`)
	return code
}

func highlightJSON(code string) string {
	code = jsonStringRE.ReplaceAllString(code, `<span class="sh-string">$0</span>`)
	code = jsonBoolRE.ReplaceAllString(code, `<span class="sh-keyword">$1</span>`)
	code = jsonNumberRE.ReplaceAllString(code, `<span class="sh-number">$1</span>`)
	return code
}

func highlightHTML(code string) string {
	code = htmlAttrRE.ReplaceAllString(code, `<span class="sh-attr">$1</span>$2`)
	code = htmlTagRE.ReplaceAllString(code, `<span class="sh-tag">$1</span>`)
	return code
}

func highlightCSS(code string) string {
	code = cssCommentRE.ReplaceAllString(code, `<span class="sh-comment">$1</span>`)
	code = cssSelectorRE.ReplaceAllString(code, `<span class="sh-keyword">$1</span>$2`)
	code = cssPropRE.ReplaceAllString(code, `<span class="sh-attr">$1</span>$2`)
	code = cssValueRE.ReplaceAllString(code, `$1<span class="sh-string">$2</span>`)
	return code
}

var goKeywords = []string{
	"break", "case", "chan", "const", "continue", "default", "defer",
	"else", "fallthrough", "for", "func", "go", "goto", "if", "import",
	"interface", "map", "package", "range", "return", "select", "struct",
	"switch", "type", "var",
}

var goBuiltins = []string{
	"append", "cap", "close", "complex", "copy", "delete", "imag",
	"len", "make", "new", "panic", "print", "println", "real", "recover",
	"bool", "byte", "complex64", "complex128", "error", "float32", "float64",
	"int", "int8", "int16", "int32", "int64", "rune", "string",
	"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
	"true", "false", "nil", "iota",
}

var jsKeywords = []string{
	"async", "await", "break", "case", "catch", "class", "const",
	"continue", "debugger", "default", "delete", "do", "else", "export",
	"extends", "false", "finally", "for", "from", "function", "if",
	"import", "in", "instanceof", "let", "new", "null", "of", "return",
	"static", "super", "switch", "this", "throw", "true", "try", "typeof",
	"undefined", "var", "void", "while", "with", "yield",
}

var pyKeywords = []string{
	"and", "as", "assert", "async", "await", "break", "class", "continue",
	"def", "del", "elif", "else", "except", "False", "finally", "for",
	"from", "global", "if", "import", "in", "is", "lambda", "None",
	"nonlocal", "not", "or", "pass", "raise", "return", "True", "try",
	"while", "with", "yield",
}

func buildKeywordRE(keywords []string) *regexp.Regexp {
	escaped := make([]string, len(keywords))
	for i, kw := range keywords {
		escaped[i] = regexp.QuoteMeta(kw)
	}
	return regexp.MustCompile(`\b(` + strings.Join(escaped, "|") + `)\b`)
}

func init() {
	plugins.Register("syntaxhighlight", func() plugins.Plugin {
		return &Plugin{}
	})
}
