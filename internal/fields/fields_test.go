package fields

import "testing"

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
