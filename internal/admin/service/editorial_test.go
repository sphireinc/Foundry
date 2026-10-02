package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/admin/users"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
)

func editorialContexts(t *testing.T, cfg *config.Config) map[string]context.Context {
	t.Helper()
	cfg.Admin.Enabled = true
	hash, err := users.HashPassword("StrongPassword123!")
	if err != nil {
		t.Fatal(err)
	}
	records := []users.User{{Username: "alice", Role: "author", PasswordHash: hash}, {Username: "bob", Role: "author", PasswordHash: hash}, {Username: "editor", Role: "editor", PasswordHash: hash}, {Username: "reviewer", Role: "reviewer", PasswordHash: hash}, {Username: "other-reviewer", Role: "reviewer", PasswordHash: hash}}
	if err := users.Save(cfg.Admin.UsersFile, records); err != nil {
		t.Fatal(err)
	}
	middleware := adminauth.New(cfg)
	contexts := map[string]context.Context{}
	for _, user := range records {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "127.0.0.1:12345"
		response := httptest.NewRecorder()
		if _, err := middleware.Login(response, request, user.Username, "StrongPassword123!", ""); err != nil {
			t.Fatal(err)
		}
		for _, cookie := range response.Result().Cookies() {
			request.AddCookie(cookie)
		}
		middleware.Wrap(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) { contexts[user.Username] = req.Context() })).ServeHTTP(httptest.NewRecorder(), request)
		if contexts[user.Username] == nil {
			t.Fatalf("missing context for %s", user.Username)
		}
	}
	return contexts
}
func editorialRead(t *testing.T, cfg *config.Config) (*content.FrontMatter, string, string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(cfg.ContentDir, "posts", "story.md"))
	if err != nil {
		t.Fatal(err)
	}
	fm, body, err := content.ParseDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := editorialRevision(fm, body)
	if err != nil {
		t.Fatal(err)
	}
	return fm, body, revision
}

func TestEditorialApprovalPublicationAndInvalidation(t *testing.T) {
	cfg := testServiceConfig(t)
	cfg.Editorial.RequireApproval = true
	contexts := editorialContexts(t, cfg)
	svc := New(cfg)
	_, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{SourcePath: "posts/story.md", Username: "alice", Raw: "---\ntitle: Story\nauthor: alice\ndraft: true\n---\n\nOriginal body"})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := svc.AcquireDocumentLock(contexts["alice"], types.DocumentLockRequest{SourcePath: "posts/story.md"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateDocumentStatus(contexts["alice"], types.DocumentStatusRequest{SourcePath: "posts/story.md", Status: "published", LockToken: lock.Lock.Token}); err == nil {
		t.Fatal("author published without approval")
	}
	if _, err := svc.UpdateDocumentStatus(contexts["editor"], types.DocumentStatusRequest{SourcePath: "posts/story.md", Status: "published"}); err == nil {
		t.Fatal("editor published without approval")
	}
	_, _, revision := editorialRead(t, cfg)
	assigned, err := svc.UpdateEditorial(contexts["editor"], types.EditorialRequest{SourcePath: "posts/story.md", Action: "assign", Assignee: "bob", Reviewer: "reviewer", ExpectedRevision: revision})
	if err != nil {
		t.Fatal(err)
	}
	if assigned.Assignee != "bob" {
		t.Fatal("assignment not saved")
	}
	detail, err := svc.GetDocument(contexts["bob"], filepath.Join(cfg.ContentDir, "posts", "story.md"), true)
	if err != nil {
		t.Fatalf("assignee cannot read: %v", err)
	}
	if _, err := json.Marshal(detail); err != nil {
		t.Fatalf("assigned document cannot be served as JSON: %v", err)
	}
	if _, err := svc.UpdateDocumentStatus(contexts["alice"], types.DocumentStatusRequest{SourcePath: "posts/story.md", Status: "in_review", LockToken: lock.Lock.Token}); err != nil {
		t.Fatal(err)
	}
	request := types.EditorialRequest{SourcePath: "posts/story.md", Action: "approve", ExpectedRevision: revision}
	if _, err := svc.UpdateEditorial(contexts["other-reviewer"], request); err == nil {
		t.Fatal("unassigned reviewer approved")
	}
	request.ExpectedRevision = "stale"
	if _, err := svc.UpdateEditorial(contexts["reviewer"], request); err == nil {
		t.Fatal("stale revision approved")
	}
	request.ExpectedRevision = revision
	approval, err := svc.UpdateEditorial(contexts["reviewer"], request)
	if err != nil {
		t.Fatal(err)
	}
	if approval.ApprovedRevision != revision || approval.ApprovedBy != "reviewer" {
		t.Fatal("revision not approved")
	}
	for _, schedule := range []struct{ publish, unpublish string }{
		{},
		{publish: "not-a-time"},
		{publish: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)},
		{publish: time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339), unpublish: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)},
	} {
		fm, body, _ := editorialRead(t, cfg)
		fm.Params["workflow"] = "scheduled"
		fm.Params["scheduled_publish_at"] = schedule.publish
		fm.Params["scheduled_unpublish_at"] = schedule.unpublish
		raw, err := marshalDocument(fm, body)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.SaveDocument(contexts["editor"], types.DocumentSaveRequest{SourcePath: "posts/story.md", Raw: string(raw)}); err == nil {
			t.Fatalf("raw save accepted invalid schedule: %#v", schedule)
		}
		if _, err := svc.UpdateDocumentStatus(contexts["editor"], types.DocumentStatusRequest{SourcePath: "posts/story.md", Status: "scheduled", ScheduledPublishAt: schedule.publish, ScheduledUnpublishAt: schedule.unpublish}); err == nil {
			t.Fatalf("status update accepted invalid schedule: %#v", schedule)
		}
	}
	// The editor's raw-save path also accepts a valid, approved schedule.
	{
		fm, body, _ := editorialRead(t, cfg)
		fm.Params["workflow"] = "scheduled"
		fm.Params["scheduled_publish_at"] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
		raw, err := marshalDocument(fm, body)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.SaveDocument(contexts["editor"], types.DocumentSaveRequest{SourcePath: "posts/story.md", Raw: string(raw)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.UpdateDocumentStatus(contexts["editor"], types.DocumentStatusRequest{SourcePath: "posts/story.md", Status: "scheduled", ScheduledPublishAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateDocumentStatus(contexts["editor"], types.DocumentStatusRequest{SourcePath: "posts/story.md", Status: "published"}); err != nil {
		t.Fatal(err)
	}
	fm, body, _ := editorialRead(t, cfg)
	// A raw save cannot forge approval or keep changed content scheduled.
	fm.Params["editorial"] = map[string]any{"approved_by": "forged", "approved_revision": "forged"}
	raw, err := marshalDocument(fm, body+"\nChanged")
	if err != nil {
		t.Fatal(err)
	}
	changed, err := svc.SaveDocument(contexts["alice"], types.DocumentSaveRequest{SourcePath: "posts/story.md", Raw: string(raw), LockToken: lock.Lock.Token})
	if err != nil {
		t.Fatal(err)
	}
	fm, _, _ = editorialRead(t, cfg)
	state, err := editorialState(fm)
	if err != nil {
		t.Fatal(err)
	}
	if state.ApprovedBy != "" || state.Reviewer != "reviewer" || content.WorkflowFromFrontMatter(fm, time.Now()).Status != "draft" {
		t.Fatalf("approval not revoked: %#v", state)
	}
	if !strings.Contains(changed.Raw, "revision_changed") {
		t.Fatal("missing review history")
	}
	if _, err := svc.UpdateDocumentStatus(contexts["editor"], types.DocumentStatusRequest{SourcePath: "posts/story.md", Status: "published"}); err == nil {
		t.Fatal("changed revision reused approval")
	}
	// Existing history and diff compare metadata separately from body.
	history, err := svc.GetDocumentHistory(contexts["editor"], "posts/story.md")
	if err != nil {
		t.Fatal(err)
	}
	var version string
	for _, entry := range history.Entries {
		if entry.State == types.LifecycleStateVersion {
			version = entry.Path
			break
		}
	}
	if version == "" {
		t.Fatal("workflow updates not versioned")
	}
	diff, err := svc.DiffDocument(contexts["editor"], types.DocumentDiffRequest{LeftPath: version, RightPath: "posts/story.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.FrontmatterChanges) == 0 || !strings.Contains(diff.BodyDiff, "Changed") {
		t.Fatal("structured comparison missing")
	}
	if _, err := svc.RestoreDocument(contexts["editor"], types.DocumentLifecycleRequest{Path: version}); err != nil {
		t.Fatal(err)
	}
	fm, _, _ = editorialRead(t, cfg)
	state, _ = editorialState(fm)
	if state.ApprovedBy != "" || !fm.Draft || state.ContentEditor != "editor" {
		t.Fatal("restore bypassed review")
	}
	if _, err := svc.UpdateDocumentStatus(contexts["editor"], types.DocumentStatusRequest{SourcePath: "posts/story.md", Status: "in_review"}); err != nil {
		t.Fatal(err)
	}
	_, _, revision = editorialRead(t, cfg)
	if _, err := svc.UpdateEditorial(contexts["editor"], types.EditorialRequest{SourcePath: "posts/story.md", Action: "approve", ExpectedRevision: revision}); err == nil {
		t.Fatal("restoring editor approved own revision")
	}

}

func TestEditorialOwnershipAndIndependentReviewer(t *testing.T) {
	cfg := testServiceConfig(t)
	cfg.Editorial.RequireApproval = true
	contexts := editorialContexts(t, cfg)
	svc := New(cfg)
	if _, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{SourcePath: "posts/story.md", Username: "editor", Raw: "---\ntitle: Story\nauthor: editor\ndraft: true\n---\n\nBody"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetDocument(contexts["alice"], filepath.Join(cfg.ContentDir, "posts", "story.md"), true); err == nil {
		t.Fatal("author read another owner's document")
	}
	if _, err := svc.GetDocumentHistory(contexts["alice"], "posts/story.md"); err == nil {
		t.Fatal("author read another owner's history")
	}
	if _, err := svc.UpdateDocumentStatus(contexts["editor"], types.DocumentStatusRequest{SourcePath: "posts/story.md", Status: "in_review"}); err != nil {
		t.Fatal(err)
	}
	_, _, revision := editorialRead(t, cfg)
	if _, err := svc.UpdateEditorial(contexts["editor"], types.EditorialRequest{SourcePath: "posts/story.md", Action: "approve", ExpectedRevision: revision}); err == nil {
		t.Fatal("owner approved own revision")
	}
	if _, err := svc.UpdateEditorial(contexts["reviewer"], types.EditorialRequest{SourcePath: "posts/story.md", Action: "request_changes", ExpectedRevision: revision, Note: "Check facts"}); err != nil {
		t.Fatal(err)
	}
	fm, _, _ := editorialRead(t, cfg)
	if !fm.Draft || fm.Params["editorial_note"] != "Check facts" {
		t.Fatal("changes request not recorded")
	}
	deleted, err := svc.DeleteDocument(contexts["editor"], types.DocumentDeleteRequest{SourcePath: "posts/story.md"})
	if err != nil {
		t.Fatal(err)
	}
	trash, err := svc.ListDocumentTrash(contexts["alice"])
	if err != nil {
		t.Fatal(err)
	}
	if len(trash) != 0 {
		t.Fatal("author can inspect another owner's trash")
	}
	if _, err := svc.PurgeDocument(contexts["alice"], types.DocumentLifecycleRequest{Path: deleted.TrashPath}); err == nil {
		t.Fatal("author purged another owner's trash")
	}

}

func TestEditorialNoOpSaveDoesNotHideContentEditor(t *testing.T) {
	cfg := testServiceConfig(t)
	cfg.Editorial.RequireApproval = true
	contexts := editorialContexts(t, cfg)
	svc := New(cfg)
	saved, err := svc.SaveDocument(context.Background(), types.DocumentSaveRequest{SourcePath: "posts/story.md", Username: "editor", Raw: "---\ntitle: Story\nauthor: alice\ndraft: true\n---\n\nEditor wrote this"})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := svc.AcquireDocumentLock(contexts["alice"], types.DocumentLockRequest{SourcePath: "posts/story.md"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveDocument(contexts["alice"], types.DocumentSaveRequest{SourcePath: "posts/story.md", Raw: saved.Raw, LockToken: lock.Lock.Token}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateDocumentStatus(contexts["alice"], types.DocumentStatusRequest{SourcePath: "posts/story.md", Status: "in_review", LockToken: lock.Lock.Token}); err != nil {
		t.Fatal(err)
	}
	fm, _, revision := editorialRead(t, cfg)
	state, err := editorialState(fm)
	if err != nil {
		t.Fatal(err)
	}
	if state.ContentEditor != "editor" {
		t.Fatalf("no-op save lost content editor: %#v", state)
	}
	if _, err := svc.UpdateEditorial(contexts["editor"], types.EditorialRequest{SourcePath: "posts/story.md", Action: "approve", ExpectedRevision: revision}); err == nil {
		t.Fatal("editor approved own content after a no-op save")
	}
}
