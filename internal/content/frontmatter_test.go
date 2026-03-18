package content

import "testing"

func TestParseDocument(t *testing.T) {
	doc := []byte("---\ntitle: Hello\nslug: hello\n---\n\nBody")
	fm, body, err := ParseDocument(doc)
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}
	if fm.Title != "Hello" || fm.Slug != "hello" || body != "\nBody" {
		t.Fatalf("unexpected parse result: %#v %q", fm, body)
	}
}

func TestParseDocumentError(t *testing.T) {
	if _, _, err := ParseDocument([]byte("---\ntitle: [\n---")); err == nil {
		t.Fatal("expected parse error")
	}
}
