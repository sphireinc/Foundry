package redirects

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseDefaultsAndNormalizesRedirects(t *testing.T) {
	store, err := Parse([]byte(`
redirects:
  - from: old-path
    to: new-path
    preserve_query: true
  - from: /external/
    to: https://example.com/target
    status: 308
    enabled: false
`))
	if err != nil {
		t.Fatalf("parse redirects: %v", err)
	}
	if len(store.Redirects) != 2 {
		t.Fatalf("expected 2 redirects, got %d", len(store.Redirects))
	}
	var first Rule
	for _, rule := range store.Redirects {
		if rule.From == "/old-path/" {
			first = rule
			break
		}
	}
	if first.From != "/old-path/" || first.To != "/new-path/" || first.Status != 301 || !first.Enabled || !first.PreserveQuery {
		t.Fatalf("unexpected normalized redirect: %#v", first)
	}
}

func TestNormalizeStoreRejectsDuplicatesAndInvalidStatus(t *testing.T) {
	if _, err := NormalizeStore(&Store{Redirects: []Rule{
		{From: "/old/", To: "/new/", Status: 301, Enabled: true},
		{From: "/old", To: "/other/", Status: 302, Enabled: true},
	}}); err == nil {
		t.Fatal("expected duplicate source error")
	}
	if _, err := NormalizeStore(&Store{Redirects: []Rule{
		{From: "/old/", To: "/new/", Status: 303, Enabled: true},
	}}); err == nil {
		t.Fatal("expected invalid status error")
	}
}

func TestMarshalRoundTripPreservesDisabledRedirects(t *testing.T) {
	store, err := NormalizeStore(&Store{Redirects: []Rule{
		{From: "/old/", To: "/new/", Status: 301, Enabled: false},
	}})
	if err != nil {
		t.Fatalf("normalize store: %v", err)
	}
	body, err := yaml.Marshal(store)
	if err != nil {
		t.Fatalf("marshal store: %v", err)
	}
	parsed, err := Parse(body)
	if err != nil {
		t.Fatalf("parse store: %v", err)
	}
	if len(parsed.Redirects) != 1 || parsed.Redirects[0].Enabled {
		t.Fatalf("expected disabled redirect to survive round trip, got %#v", parsed.Redirects)
	}
}

func TestLookupAndTargetWithQuery(t *testing.T) {
	rule, ok := Lookup([]Rule{
		{From: "/old/", To: "/new/", Status: 301, Enabled: false},
		{From: "/kept/", To: "/target/", Status: 302, Enabled: true, PreserveQuery: true},
	}, "/kept")
	if !ok {
		t.Fatal("expected redirect match")
	}
	if got := TargetWithQuery(rule, "utm=1"); got != "/target/?utm=1" {
		t.Fatalf("unexpected target with query: %q", got)
	}
	rule.To = "/target/#section"
	if got := TargetWithQuery(rule, "utm=1"); got != "/target/?utm=1#section" {
		t.Fatalf("unexpected fragment target with query: %q", got)
	}
	if _, ok := Lookup([]Rule{{From: "/old/", To: "/new/", Status: 301, Enabled: false}}, "/old/"); ok {
		t.Fatal("disabled redirect should not match")
	}
}
