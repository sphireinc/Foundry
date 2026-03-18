package data

import "testing"

func TestStoreSetGetAll(t *testing.T) {
	store := New()
	store.Set("nav", map[string]any{"title": "Home"})

	value, ok := store.Get("nav")
	if !ok {
		t.Fatal("expected stored key to exist")
	}
	if value.(map[string]any)["title"] != "Home" {
		t.Fatalf("unexpected value: %#v", value)
	}

	all := store.All()
	if len(all) != 1 {
		t.Fatalf("expected one value, got %d", len(all))
	}
}
