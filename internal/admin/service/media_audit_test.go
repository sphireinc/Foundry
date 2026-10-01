package service

import (
	"context"
	"testing"

	"github.com/sphireinc/foundry/internal/admin/types"
)

func TestMediaAccessibilityMetadata(t *testing.T) {
	cfg := testServiceConfig(t)
	svc := New(cfg)
	upload, err := svc.SaveMedia(context.Background(), "images", "", "tree.png", "image/png", testPNGBytes())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Media.RequireAlt = true
	if _, err := svc.SaveMediaMetadata(context.Background(), upload.Reference, types.MediaMetadata{Title: "Tree"}, "", "admin"); err == nil {
		t.Fatal("empty alt accepted in strict mode")
	}
	detail, err := svc.SaveMediaMetadata(context.Background(), upload.Reference, types.MediaMetadata{Decorative: true}, "", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if !detail.Metadata.Decorative {
		t.Fatal("decorative flag not persisted")
	}
	detail, err = svc.SaveMediaMetadata(context.Background(), upload.Reference, types.MediaMetadata{Alt: "A green tree"}, "", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Metadata.Decorative || detail.Metadata.ContentHash == "" {
		t.Fatal("editable fields or technical metadata corrupted")
	}
}
