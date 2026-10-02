package plugin_wordcount_compiled

import (
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
	"strings"
)

type Plugin struct{}

func (*Plugin) Name() string { return "wordcount-compiled" }
func (*Plugin) OnContext(ctx *renderer.ViewData) error {
	words := 0
	if ctx.Page != nil {
		words = len(strings.Fields(ctx.Page.RawBody))
	}
	if ctx.Data == nil {
		ctx.Data = map[string]any{}
	}
	ctx.Data["wordcount_compiled"] = map[string]any{"words": words}
	return nil
}
func init() { plugins.Register("wordcount-compiled", func() plugins.Plugin { return &Plugin{} }) }
