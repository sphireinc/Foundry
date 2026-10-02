package sitesearch

import (
	"encoding/json"
	"github.com/sphireinc/foundry/internal/content"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSharedContract(t *testing.T) {
	body, err := os.ReadFile("../../tests/fixtures/search/contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Entries []Entry `json:"entries"`
		Cases   []struct {
			Q, Lang, Type string
			Limit         int
			URLs          []string `json:"urls"`
			Total         int
		} `json:"cases"`
	}
	if err = json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, tc := range fixture.Cases {
		result := Query(fixture.Entries, tc.Q, Options{Lang: tc.Lang, Type: tc.Type, Limit: tc.Limit})
		urls := []string{}
		for _, item := range result.Items {
			urls = append(urls, item.URL)
		}
		if !reflect.DeepEqual(urls, tc.URLs) || result.Total != tc.Total {
			t.Fatalf("query %q: %+v", tc.Q, result)
		}
	}
}
func TestPublicationSafety(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name        string
		params      map[string]any
		draft, want bool
	}{
		{"published", nil, false, true},
		{"draft", nil, true, false},
		{"review", map[string]any{"workflow": "in_review"}, false, false},
		{"archived", map[string]any{"archived": true}, false, false},
		{"future", map[string]any{"workflow": "scheduled", "scheduled_publish_at": now.Add(time.Hour)}, false, false},
		{"unscheduled", map[string]any{"workflow": "scheduled"}, false, false},
		{"expired", map[string]any{"scheduled_unpublish_at": now.Add(-time.Hour)}, false, false},
	} {
		if got := Public(&content.Document{Draft: tc.draft, Params: tc.params}, now); got != tc.want {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}
func TestLimits(t *testing.T) {
	for input, want := range map[string]int{"": 20, "0": 20, "-1": 20, "abc": 20, "2": 2, "999": 100, "1e2": 20} {
		if got := ParseLimit(input); got != want {
			t.Errorf("%q: %d", input, got)
		}
	}
}

func TestSnippetUnicodeOffsets(t *testing.T) {
	body := strings.Repeat("Ⱥ", 100) + "needle" + strings.Repeat("é", 200)
	got := Snippet("", "", body, "needle")
	if !strings.Contains(got, "needle") || !strings.HasPrefix(got, "...") {
		t.Fatalf("unexpected snippet: %q", got)
	}
}
