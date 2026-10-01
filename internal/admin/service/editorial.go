package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/admin/users"
	"github.com/sphireinc/foundry/internal/content"
	"gopkg.in/yaml.v3"
)

// Editorial metadata is server-owned. Its revision excludes workflow controls,
// timestamps and audit data, but includes the document's content and other fields.
func editorialState(fm *content.FrontMatter) (types.EditorialState, error) {
	state := types.EditorialState{Events: []types.EditorialEvent{}}
	if fm == nil || fm.Params == nil || fm.Params["editorial"] == nil {
		return state, nil
	}
	body, err := yaml.Marshal(fm.Params["editorial"])
	if err != nil {
		return state, err
	}
	err = yaml.Unmarshal(body, &state)
	return state, err
}
func setEditorial(fm *content.FrontMatter, state types.EditorialState) {
	if fm.Params == nil {
		fm.Params = map[string]any{}
	}
	fm.Params["editorial"] = state
}
func editorialRevision(fm *content.FrontMatter, body string) (string, error) {
	copyFM := *fm
	copyFM.Params = map[string]any{}
	for key, value := range fm.Params {
		switch key {
		case "editorial", "workflow", "archived", "editorial_note", "scheduled_publish_at", "scheduled_unpublish_at", "version_comment", "version_actor", "versioned_at":
			continue
		}
		copyFM.Params[key] = value
	}
	copyFM.Draft = false
	copyFM.CreatedAt, copyFM.UpdatedAt = nil, nil
	copyFM.LastEditor = ""
	raw, err := marshalDocument(&copyFM, body)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}
func exactCapability(identity *adminauth.Identity, capability string) bool {
	if identity == nil {
		return false
	}
	for _, value := range identity.Capabilities {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "*" || value == capability {
			return true
		}
	}
	return false
}
func canEditFrontMatter(identity *adminauth.Identity, fm *content.FrontMatter) bool {
	if canMutateDocument(identity, documentOwnerFromFrontMatter(fm)) {
		return true
	}
	state, err := editorialState(fm)
	return err == nil && exactCapability(identity, "documents.write.own") && strings.EqualFold(state.Assignee, identity.Username)
}
func editorialActor(ctx context.Context) string {
	if identity, ok := currentIdentity(ctx); ok {
		return identity.Username
	}
	return "trusted-service"
}
func appendEditorialEvent(state *types.EditorialState, action, actor, revision, note string) {
	state.Events = append(state.Events, types.EditorialEvent{Action: action, Actor: actor, At: time.Now().UTC(), Revision: revision, Note: strings.TrimSpace(note)})
}
func revokeApproval(state *types.EditorialState) { state.ApprovedBy, state.ApprovedRevision = "", "" }

func (s *Service) editorialPublicationAllowed(ctx context.Context, fm *content.FrontMatter, body, status string) error {
	if !s.cfg.Editorial.RequireApproval || (status != "published" && status != "scheduled") {
		return nil
	}
	identity, ok := currentIdentity(ctx)
	if !ok || !exactCapability(identity, "documents.publish") {
		return fmt.Errorf("publication requires documents.publish")
	}
	state, err := editorialState(fm)
	if err != nil {
		return err
	}
	revision, err := editorialRevision(fm, body)
	if err != nil {
		return err
	}
	if state.ApprovedBy == "" || state.ApprovedRevision != revision {
		return fmt.Errorf("publication requires approval of the current revision")
	}
	return nil
}

func (s *Service) prepareEditorialSave(ctx context.Context, fm *content.FrontMatter, body string, previous *content.FrontMatter, previousBody string) error {
	// Clients cannot manufacture or overwrite assignments, approvals or history.
	delete(fm.Params, "editorial")
	state, err := editorialState(previous)
	if err != nil {
		return err
	}
	if previous != nil {
		if state.ContentEditor == "" {
			state.ContentEditor = previous.LastEditor
		}
		if identity, ok := currentIdentity(ctx); ok && !canEditFrontMatter(identity, previous) {
			return fmt.Errorf("document access denied")
		}
		if documentOwnerFromFrontMatter(previous) != "" && documentOwnerFromFrontMatter(fm) != documentOwnerFromFrontMatter(previous) {
			return fmt.Errorf("change ownership through the editorial assignment action")
		}
	}
	revision, err := editorialRevision(fm, body)
	if err != nil {
		return err
	}
	oldRevision := ""
	if previous != nil {
		oldRevision, err = editorialRevision(previous, previousBody)
		if err != nil {
			return err
		}
	}
	status := content.WorkflowFromFrontMatter(fm, time.Now().UTC()).Status
	oldStatus := ""
	if previous != nil {
		oldStatus = content.WorkflowFromFrontMatter(previous, time.Now().UTC()).Status
	}
	changed := previous != nil && oldRevision != revision
	if previous == nil || changed {
		state.ContentEditor = fm.LastEditor
		if state.ContentEditor == "" {
			state.ContentEditor = editorialActor(ctx)
		}
	}
	if changed {
		revokeApproval(&state)
		appendEditorialEvent(&state, "revision_changed", editorialActor(ctx), revision, "")
		if s.cfg.Editorial.RequireApproval && (oldStatus == "published" || oldStatus == "scheduled" || oldStatus == "in_review") && status == oldStatus {
			content.ApplyWorkflowToFrontMatter(fm, "draft", nil, nil, "")
			status = "draft"
		}
	}
	if s.cfg.Editorial.RequireApproval && previous != nil {
		identity, ok := currentIdentity(ctx)
		if ok && !exactCapability(identity, "documents.publish") {
			for _, key := range []string{"scheduled_publish_at", "scheduled_unpublish_at"} {
				if fmt.Sprint(fm.Params[key]) != fmt.Sprint(previous.Params[key]) && fm.Params[key] != nil {
					return fmt.Errorf("publication scheduling requires documents.publish")
				}
			}
		}
	}
	if previous == nil && s.cfg.Editorial.RequireApproval && status == "published" {
		content.ApplyWorkflowToFrontMatter(fm, "draft", nil, nil, "")
		status = "draft"
	}
	if status != oldStatus {
		if status == "draft" || status == "in_review" || status == "archived" {
			revokeApproval(&state)
		}
		appendEditorialEvent(&state, status, editorialActor(ctx), revision, "")
	}
	setEditorial(fm, state)
	return s.editorialPublicationAllowed(ctx, fm, body, status)
}

// UpdateEditorial operates on a saved revision, never an unsaved editor buffer.
func (s *Service) UpdateEditorial(ctx context.Context, req types.EditorialRequest) (*types.EditorialState, error) {
	s.documentMu.Lock()
	defer s.documentMu.Unlock()
	identity, ok := currentIdentity(ctx)
	if !ok || identity.Username == "" {
		return nil, fmt.Errorf("editorial actions require an authenticated identity")
	}
	if len(req.Note) > 4000 {
		return nil, fmt.Errorf("editorial note exceeds 4000 bytes")
	}
	path, err := s.resolveContentPath(req.SourcePath)
	if err != nil {
		return nil, err
	}
	raw, err := s.fs.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fm, body, err := content.ParseDocument(raw)
	if err != nil {
		return nil, err
	}
	if !canAccessDocument(identity, &content.Document{Author: fm.Author, Params: fm.Params}) {
		return nil, fmt.Errorf("document access denied")
	}
	if err := s.ensureDocumentLock(ctx, req.SourcePath, req.LockToken); err != nil {
		return nil, err
	}
	revision, err := editorialRevision(fm, body)
	if err != nil {
		return nil, err
	}
	if req.ExpectedRevision == "" || req.ExpectedRevision != revision {
		return nil, fmt.Errorf("document revision changed; reload before making an editorial decision")
	}
	state, err := editorialState(fm)
	if err != nil {
		return nil, err
	}
	switch req.Action {
	case "assign":
		if !exactCapability(identity, "documents.assign") {
			return nil, fmt.Errorf("assignment requires documents.assign")
		}
		for i, username := range []string{req.Owner, req.Assignee, req.Reviewer} {
			if err := s.validateEditorialUser(username, i == 2); err != nil {
				return nil, err
			}
		}
		ownerChanged := req.Owner != "" && !strings.EqualFold(req.Owner, documentOwnerFromFrontMatter(fm))
		if ownerChanged {
			state.ContentEditor = identity.Username
			fm.Author = strings.TrimSpace(req.Owner)
			delete(fm.Params, "owner")
		}
		if ownerChanged || !strings.EqualFold(state.Assignee, strings.TrimSpace(req.Assignee)) || !strings.EqualFold(state.Reviewer, strings.TrimSpace(req.Reviewer)) {
			revokeApproval(&state)
			status := content.WorkflowFromFrontMatter(fm, time.Now().UTC()).Status
			if s.cfg.Editorial.RequireApproval && (status == "published" || status == "scheduled") {
				content.ApplyWorkflowToFrontMatter(fm, "draft", nil, nil, "")
			}
		}
		state.Assignee, state.Reviewer = strings.TrimSpace(req.Assignee), strings.TrimSpace(req.Reviewer)
	case "approve", "request_changes":
		if !exactCapability(identity, "documents.review") {
			return nil, fmt.Errorf("review requires documents.review")
		}
		if content.WorkflowFromFrontMatter(fm, time.Now().UTC()).Status != "in_review" {
			return nil, fmt.Errorf("submit the document for review before deciding")
		}
		if state.Reviewer != "" && !strings.EqualFold(state.Reviewer, identity.Username) {
			return nil, fmt.Errorf("only the assigned reviewer may decide")
		}
		if strings.EqualFold(identity.Username, documentOwnerFromFrontMatter(fm)) || strings.EqualFold(identity.Username, state.Assignee) || strings.EqualFold(identity.Username, state.ContentEditor) || (state.ContentEditor == "" && strings.EqualFold(identity.Username, fm.LastEditor)) {
			return nil, fmt.Errorf("an independent reviewer must decide on this revision")
		}
		if req.Action == "approve" {
			state.ApprovedBy, state.ApprovedRevision = identity.Username, revision
		} else {
			revokeApproval(&state)
			content.ApplyWorkflowToFrontMatter(fm, "draft", nil, nil, req.Note)
		}
	default:
		return nil, fmt.Errorf("editorial action must be assign, approve, or request_changes")
	}
	revision, err = editorialRevision(fm, body)
	if err != nil {
		return nil, err
	}
	appendEditorialEvent(&state, req.Action, identity.Username, revision, req.Note)
	setEditorial(fm, state)
	fm.UpdatedAt = timePointer(time.Now().UTC())
	rendered, err := marshalDocument(fm, body)
	if err != nil {
		return nil, err
	}
	if err := s.snapshotDocumentVersion(path, time.Now(), "editorial: "+req.Action, identity.Username); err != nil {
		return nil, err
	}
	if err := s.fs.WriteFile(path, rendered, 0o644); err != nil {
		return nil, err
	}
	s.invalidateGraphCache()
	state.Revision, state.RequireApproval = revision, s.cfg.Editorial.RequireApproval
	return &state, nil
}
func timePointer(value time.Time) *time.Time { return &value }
func (s *Service) validateEditorialUser(username string, reviewer bool) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}
	list, err := users.Load(s.cfg.Admin.UsersFile)
	if err != nil {
		return err
	}
	for _, user := range list {
		if !user.Disabled && strings.EqualFold(user.Username, username) {
			if reviewer {
				role := strings.ToLower(strings.TrimSpace(user.Role))
				capable := role == "admin" || role == "editor" || role == "reviewer"
				for _, capability := range user.Capabilities {
					capable = capable || capability == "*" || capability == "documents.review"
				}
				if !capable {
					return fmt.Errorf("assigned reviewer requires documents.review: %s", username)
				}
			}
			return nil
		}
	}
	return fmt.Errorf("editorial assignment requires an active user: %s", username)
}

// authorizeRevision keeps own-scoped history/diff/lifecycle requests within
// documents the caller can access, including assignment-based access.
func (s *Service) authorizeRevision(ctx context.Context, path, capability string) error {
	identity, ok := currentIdentity(ctx)
	if !ok {
		return nil
	}
	if exactCapability(identity, capability) {
		return nil
	}
	if !exactCapability(identity, capability+".own") {
		return fmt.Errorf("insufficient capability: %s", capability)
	}
	_, original, _, err := s.resolveDocumentLifecyclePath(path)
	if err != nil {
		return err
	}
	raw, err := s.fs.ReadFile(original)
	if os.IsNotExist(err) {
		raw, err = s.fs.ReadFile(path)
	}
	if err != nil {
		return err
	}
	fm, _, err := content.ParseDocument(raw)
	if err != nil {
		return err
	}
	if !canAccessDocument(identity, &content.Document{Author: fm.Author, Params: fm.Params}) {
		return fmt.Errorf("document access denied")
	}
	return nil
}
