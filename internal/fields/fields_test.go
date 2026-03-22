package fields

import (
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestNormalizeReturnsEmptyMapForNil(t *testing.T) {
	got := Normalize(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty map, got %#v", got)
	}
}

func TestNormalizeReturnsInputMap(t *testing.T) {
	in := map[string]any{"x": 1}
	got := Normalize(in)
	if got["x"] != 1 {
		t.Fatalf("expected existing map to be returned, got %#v", got)
	}
}

func TestApplyDefaultsAndValidate(t *testing.T) {
	defs := []Definition{
		{Name: "hero", Type: "text", Required: true, Default: "launch"},
		{Name: "featured", Type: "bool", Default: true},
		{Name: "stage", Type: "select", Enum: []string{"draft", "review"}},
		{Name: "seo", Type: "object", Fields: []config.FieldDefinition{
			{Name: "title", Type: "text", Required: true, Default: "Foundry"},
		}},
	}

	values := ApplyDefaults(map[string]any{"stage": "draft"}, defs)
	if values["hero"] != "launch" {
		t.Fatalf("expected default hero field, got %#v", values["hero"])
	}
	if values["featured"] != true {
		t.Fatalf("expected default featured field, got %#v", values["featured"])
	}
	seo, ok := values["seo"].(map[string]any)
	if !ok || seo["title"] != "Foundry" {
		t.Fatalf("expected default nested field, got %#v", values["seo"])
	}
	if errs := Validate(values, defs, false); len(errs) != 0 {
		t.Fatalf("expected valid fields, got %v", errs)
	}
	if errs := Validate(map[string]any{"stage": "nope"}, defs, false); len(errs) == 0 {
		t.Fatal("expected invalid enum error")
	}
}
