package callout

import (
	"html/template"

	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/renderer"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "callout"
}

func (p *Plugin) OnHTMLSlots(ctx *renderer.ViewData, slots *renderer.Slots) error {
	if ctx.Page == nil || ctx.Page.Type != "post" {
		return nil
	}

	slots.Add("post.after_content", template.HTML(`
<section class="content-panel" style="margin-top:1.25rem;">
  <div class="eyebrow">Plugin</div>
  <h2 style="margin-top:0;">Enjoying this article?</h2>
  <p class="muted">This block was injected by a plugin through the theme slot system.</p>
</section>
`))

	return nil
}

func init() {
	plugins.Register("callout", func() plugins.Plugin {
		return &Plugin{}
	})
}
