package cache

import "testing"

func TestMemoryStoreCRUD(t *testing.T) {
	store := NewMemoryStore()

	if _, ok := store.Get("missing"); ok {
		t.Fatal("expected missing key to be absent")
	}

	store.Set("a", 1)
	if v, ok := store.Get("a"); !ok || v.(int) != 1 {
		t.Fatalf("expected stored value, got %v %v", v, ok)
	}

	store.Delete("a")
	if _, ok := store.Get("a"); ok {
		t.Fatal("expected deleted key to be absent")
	}

	store.Set("a", 1)
	store.Set("b", 2)
	store.Clear()
	if _, ok := store.Get("a"); ok {
		t.Fatal("expected clear to remove values")
	}
	if _, ok := store.Get("b"); ok {
		t.Fatal("expected clear to remove values")
	}
}
