package plugin_wordcount_compiled

import (
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/renderer"
	"testing"
)

func TestWordCount(t *testing.T) {
	for _, tc := range []struct {
		page  *content.Document
		words int
	}{{nil, 0}, {&content.Document{RawBody: "one two three"}, 3}} {
		ctx := &renderer.ViewData{Page: tc.page}
		if err := (&Plugin{}).OnContext(ctx); err != nil {
			t.Fatal(err)
		}
		if got := ctx.Data["wordcount_compiled"].(map[string]any)["words"]; got != tc.words {
			t.Fatalf("got %v want %d", got, tc.words)
		}
	}
}
