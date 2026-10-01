package content

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScheduledWorkflowPublicationWindow(t *testing.T) {
	publish := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	unpublish := publish.Add(time.Hour)
	fm := &FrontMatter{Draft: true, Params: map[string]any{"workflow": "scheduled", "scheduled_publish_at": publish.Format(time.RFC3339), "scheduled_unpublish_at": unpublish.Format(time.RFC3339)}}
	for _, test := range []struct {
		now    time.Time
		status string
	}{{publish.Add(-time.Second), "scheduled"}, {publish, "published"}, {publish.Add(time.Second), "published"}, {unpublish, "draft"}, {unpublish.Add(time.Second), "draft"}} {
		if got := WorkflowFromFrontMatter(fm, test.now).Status; got != test.status {
			t.Fatalf("at %s got %s want %s", test.now, got, test.status)
		}
	}
	fm.Params["workflow"] = "in_review"
	if got := WorkflowFromFrontMatter(fm, publish.Add(time.Second)).Status; got != "in_review" {
		t.Fatalf("timestamp bypassed review: %s", got)
	}
}

func TestLoaderPublishesDueScheduleWithStoredDraftFlag(t *testing.T) {
	cfg := testLoaderConfig(t)
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	raw := "---\ntitle: Due Story\nslug: due-story\ndraft: true\nworkflow: scheduled\nscheduled_publish_at: " + past + "\n---\n\nBody"
	if err := os.WriteFile(filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "due.md"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	graph, err := NewLoader(cfg, nil, false).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range graph.Documents {
		if doc.Slug == "due-story" {
			if doc.Draft || doc.Status != "published" {
				t.Fatal("due document remained draft")
			}
			return
		}
	}
	t.Fatal("due scheduled document absent from public graph")
}
