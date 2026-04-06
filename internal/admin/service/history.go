package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/lifecycle"
	"github.com/sphireinc/foundry/internal/media"
	"gopkg.in/yaml.v3"
)

func (s *Service) GetDocumentHistory(ctx context.Context, sourcePath string) (*types.DocumentHistoryResponse, error) {
	if err := requireCapability(ctx, "documents.history"); err != nil {
		return nil, err
	}

	fullPath, originalPath, _, err := s.resolveDocumentLifecyclePath(sourcePath)
	if err != nil {
		return nil, err
	}
	entries, err := s.listDocumentLifecycleEntries(originalPath)
	if err != nil {
		return nil, err
	}
	respPath := displayDocumentPath(originalPath, s.cfg.ContentDir)
	if lifecycle.IsDerivedPath(fullPath) {
		respPath = displayDocumentPath(fullPath, s.cfg.ContentDir)
	}
	return &types.DocumentHistoryResponse{
		SourcePath: respPath,
		Entries:    entries,
	}, nil
}

func (s *Service) ListDocumentTrash(ctx context.Context) ([]types.DocumentHistoryEntry, error) {
	if err := requireCapability(ctx, "documents.history"); err != nil {
		return nil, err
	}

	entries := make([]types.DocumentHistoryEntry, 0)
	err := s.walkDir(s.cfg.ContentDir, func(path string, info os.DirEntry) error {
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".md" || !lifecycle.IsTrashPath(path) {
			return nil
		}
		entry, err := s.documentHistoryEntry(path)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortDocumentHistoryEntries(entries)
	return entries, nil
}

func (s *Service) RestoreDocument(ctx context.Context, req types.DocumentLifecycleRequest) (*types.DocumentLifecycleResponse, error) {
	if err := requireCapability(ctx, "documents.lifecycle"); err != nil {
		return nil, err
	}

	path, originalPath, state, err := s.resolveDocumentLifecyclePath(req.Path)
	if err != nil {
		return nil, err
	}
	if state == lifecycle.StateCurrent {
		return nil, fmt.Errorf("restore requires a versioned or trashed document")
	}
	if _, err := s.fs.Stat(originalPath); err == nil {
		if err := s.versionFile(originalPath, time.Now()); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := s.fs.Rename(path, originalPath); err != nil {
		return nil, err
	}
	s.invalidateGraphCache()
	return &types.DocumentLifecycleResponse{
		Path:         displayDocumentPath(path, s.cfg.ContentDir),
		RestoredPath: displayDocumentPath(originalPath, s.cfg.ContentDir),
		Operation:    "restore",
	}, nil
}

func (s *Service) PurgeDocument(ctx context.Context, req types.DocumentLifecycleRequest) (*types.DocumentLifecycleResponse, error) {
	if err := requireCapability(ctx, "documents.lifecycle"); err != nil {
		return nil, err
	}

	path, _, state, err := s.resolveDocumentLifecyclePath(req.Path)
	if err != nil {
		return nil, err
	}
	if state == lifecycle.StateCurrent {
		return nil, fmt.Errorf("purge requires a versioned or trashed document")
	}
	if err := s.fs.Remove(path); err != nil {
		return nil, err
	}
	s.invalidateGraphCache()
	return &types.DocumentLifecycleResponse{
		Path:      displayDocumentPath(path, s.cfg.ContentDir),
		Operation: "purge",
	}, nil
}

func (s *Service) DiffDocument(ctx context.Context, req types.DocumentDiffRequest) (*types.DocumentDiffResponse, error) {
	if err := requireCapability(ctx, "documents.diff"); err != nil {
		return nil, err
	}

	leftPath, _, _, err := s.resolveDocumentLifecyclePath(req.LeftPath)
	if err != nil {
		return nil, err
	}
	rightPath, _, _, err := s.resolveDocumentLifecyclePath(req.RightPath)
	if err != nil {
		return nil, err
	}

	leftBody, err := s.fs.ReadFile(leftPath)
	if err != nil {
		return nil, err
	}
	rightBody, err := s.fs.ReadFile(rightPath)
	if err != nil {
		return nil, err
	}

	return &types.DocumentDiffResponse{
		LeftPath:  displayDocumentPath(leftPath, s.cfg.ContentDir),
		RightPath: displayDocumentPath(rightPath, s.cfg.ContentDir),
		LeftRaw:   string(leftBody),
		RightRaw:  string(rightBody),
		Diff:      buildUnifiedLineDiff(leftPath, leftBody, rightPath, rightBody),
	}, nil
}

func (s *Service) GetMediaHistory(ctx context.Context, identifier string) (*types.MediaHistoryResponse, error) {
	if err := requireCapability(ctx, "media.read"); err != nil {
		return nil, err
	}

	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, fmt.Errorf("media identifier is required")
	}

	var (
		currentPath string
		reference   string
		err         error
	)
	if strings.HasPrefix(identifier, media.ReferenceScheme) {
		_, currentPath, err = s.resolveMediaItem(identifier)
		if err != nil {
			return nil, err
		}
		reference = identifier
	} else {
		_, originalPath, _, err := s.resolveMediaLifecyclePath(identifier)
		if err != nil {
			return nil, err
		}
		currentPath = originalPath
		_, reference, _, err = s.mediaReferenceInfoForPath(currentPath)
		if err != nil {
			return nil, err
		}
	}
	entries, err := s.listMediaLifecycleEntries(currentPath)
	if err != nil {
		return nil, err
	}
	return &types.MediaHistoryResponse{
		Reference: reference,
		Path:      displayDocumentPath(currentPath, s.cfg.ContentDir),
		Entries:   entries,
	}, nil
}

func (s *Service) ListMediaTrash(ctx context.Context) ([]types.MediaHistoryEntry, error) {
	if err := requireCapability(ctx, "media.read"); err != nil {
		return nil, err
	}

	entries := make([]types.MediaHistoryEntry, 0)
	for _, collection := range []string{"images", "videos", "audio", "documents", "uploads", "assets"} {
		root, err := s.mediaRoot(collection)
		if err != nil {
			return nil, err
		}
		if _, err := s.fs.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err = s.walkDir(root, func(path string, info os.DirEntry) error {
			if info.IsDir() {
				return nil
			}
			if isMediaMetadataFile(path) || !lifecycle.IsTrashPath(path) {
				return nil
			}
			entry, err := s.mediaHistoryEntry(path)
			if err != nil {
				return nil
			}
			entries = append(entries, entry)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sortMediaHistoryEntries(entries)
	return entries, nil
}

func (s *Service) RestoreMedia(ctx context.Context, req types.MediaLifecycleRequest) (*types.MediaLifecycleResponse, error) {
	if err := requireCapability(ctx, "media.lifecycle"); err != nil {
		return nil, err
	}

	path, originalPath, state, err := s.resolveMediaLifecyclePath(req.Path)
	if err != nil {
		return nil, err
	}
	if state == lifecycle.StateCurrent {
		return nil, fmt.Errorf("restore requires a versioned or trashed media file")
	}
	now := time.Now()
	if isMediaMetadataFile(path) {
		if _, err := s.fs.Stat(originalPath); err == nil {
			if err := s.snapshotMediaMetadataVersion(strings.TrimSuffix(originalPath, ".meta.yaml"), now, "", ""); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		if err := s.fs.Rename(path, originalPath); err != nil {
			return nil, err
		}
		return &types.MediaLifecycleResponse{
			Path:         displayDocumentPath(path, s.cfg.ContentDir),
			RestoredPath: displayDocumentPath(originalPath, s.cfg.ContentDir),
			Operation:    "restore",
		}, nil
	}
	if _, err := s.fs.Stat(originalPath); err == nil {
		if err := s.versionFile(originalPath, now); err != nil {
			return nil, err
		}
		if err := s.versionMediaMetadataForPrimary(originalPath, now); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := s.fs.Rename(path, originalPath); err != nil {
		return nil, err
	}
	if err := s.restoreMediaMetadata(path, originalPath); err != nil {
		return nil, err
	}

	return &types.MediaLifecycleResponse{
		Path:         displayDocumentPath(path, s.cfg.ContentDir),
		RestoredPath: displayDocumentPath(originalPath, s.cfg.ContentDir),
		Operation:    "restore",
	}, nil
}

func (s *Service) PurgeMedia(ctx context.Context, req types.MediaLifecycleRequest) (*types.MediaLifecycleResponse, error) {
	if err := requireCapability(ctx, "media.lifecycle"); err != nil {
		return nil, err
	}

	path, _, state, err := s.resolveMediaLifecyclePath(req.Path)
	if err != nil {
		return nil, err
	}
	if state == lifecycle.StateCurrent {
		return nil, fmt.Errorf("purge requires a versioned or trashed media file")
	}
	if err := s.fs.Remove(path); err != nil {
		return nil, err
	}
	if !isMediaMetadataFile(path) {
		sidecar := mediaMetadataPath(path)
		if err := s.fs.Remove(sidecar); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return &types.MediaLifecycleResponse{
		Path:      displayDocumentPath(path, s.cfg.ContentDir),
		Operation: "purge",
	}, nil
}

func (s *Service) listDocumentLifecycleEntries(originalPath string) ([]types.DocumentHistoryEntry, error) {
	dir := filepath.Dir(originalPath)
	entries, err := s.fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	out := make([]types.DocumentHistoryEntry, 0, len(entries)+1)
	if _, err := s.fs.Stat(originalPath); err == nil {
		entry, err := s.documentHistoryEntry(originalPath)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		parsedOriginal, _, _, ok := lifecycle.ParsePathDetails(fullPath)
		if !ok || filepath.Clean(parsedOriginal) != filepath.Clean(originalPath) {
			continue
		}
		historyEntry, err := s.documentHistoryEntry(fullPath)
		if err != nil {
			return nil, err
		}
		out = append(out, historyEntry)
	}
	sortDocumentHistoryEntries(out)
	return out, nil
}

func (s *Service) documentHistoryEntry(path string) (types.DocumentHistoryEntry, error) {
	info, err := s.fs.Stat(path)
	if err != nil {
		return types.DocumentHistoryEntry{}, err
	}

	body, err := s.fs.ReadFile(path)
	if err != nil {
		return types.DocumentHistoryEntry{}, err
	}
	fm, _, err := content.ParseDocument(body)
	if err != nil {
		return types.DocumentHistoryEntry{}, err
	}

	originalPath, state, ts, ok := lifecycle.ParsePathDetails(path)
	if !ok {
		originalPath = path
		state = lifecycle.StateCurrent
	}
	var timestamp *time.Time
	if !ts.IsZero() {
		timestamp = &ts
	}
	workflow := content.WorkflowFromFrontMatter(fm, time.Now().UTC())

	return types.DocumentHistoryEntry{
		Path:           displayDocumentPath(path, s.cfg.ContentDir),
		OriginalPath:   displayDocumentPath(originalPath, s.cfg.ContentDir),
		State:          toLifecycleState(state),
		Timestamp:      timestamp,
		VersionComment: versionCommentFromFrontMatter(fm),
		Actor:          versionActorFromFrontMatter(fm),
		Status:         workflow.Status,
		Title:          strings.TrimSpace(fm.Title),
		Slug:           strings.TrimSpace(fm.Slug),
		Layout:         strings.TrimSpace(fm.Layout),
		Summary:        strings.TrimSpace(fm.Summary),
		Draft:          fm.Draft,
		Archived:       documentArchivedFromParams(fm.Params),
		Lang:           documentLangFromFrontMatter(fm, s.cfg.DefaultLang),
		Author:         strings.TrimSpace(fm.Author),
		LastEditor:     strings.TrimSpace(fm.LastEditor),
		CreatedAt:      fm.CreatedAt,
		UpdatedAt:      fm.UpdatedAt,
		Size:           info.Size(),
	}, nil
}

func (s *Service) listMediaLifecycleEntries(originalPath string) ([]types.MediaHistoryEntry, error) {
	dir := filepath.Dir(originalPath)
	entries, err := s.fs.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	out := make([]types.MediaHistoryEntry, 0, len(entries)+1)
	if _, err := s.fs.Stat(originalPath); err == nil {
		entry, err := s.mediaHistoryEntry(originalPath)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	currentMetadataPath := mediaMetadataPath(originalPath)
	if _, err := s.fs.Stat(currentMetadataPath); err == nil {
		entry, err := s.mediaHistoryEntry(currentMetadataPath)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		parsedOriginal, _, _, ok := lifecycle.ParsePathDetails(fullPath)
		if !ok {
			continue
		}
		if filepath.Clean(parsedOriginal) != filepath.Clean(originalPath) && filepath.Clean(parsedOriginal) != filepath.Clean(currentMetadataPath) {
			continue
		}
		historyEntry, err := s.mediaHistoryEntry(fullPath)
		if err != nil {
			return nil, err
		}
		out = append(out, historyEntry)
	}
	sortMediaHistoryEntries(out)
	return out, nil
}

func (s *Service) mediaHistoryEntry(path string) (types.MediaHistoryEntry, error) {
	if isMediaMetadataFile(path) {
		return s.mediaMetadataHistoryEntry(path)
	}
	info, err := s.fs.Stat(path)
	if err != nil {
		return types.MediaHistoryEntry{}, err
	}
	originalPath, state, ts, ok := lifecycle.ParsePathDetails(path)
	if !ok {
		originalPath = path
		state = lifecycle.StateCurrent
	}
	metadata, versionComment, actor, err := s.loadMediaHistoryMetadata(path)
	if err != nil {
		// History/trash views should remain usable even when an optional sidecar is
		// missing or malformed.
		metadata = types.MediaMetadata{}
		versionComment = ""
		actor = ""
	}
	ref, currentRef, resolved, err := s.mediaHistoryReferenceInfo(path, originalPath, state)
	if err != nil {
		return types.MediaHistoryEntry{}, err
	}
	var timestamp *time.Time
	if !ts.IsZero() {
		timestamp = &ts
	}

	return types.MediaHistoryEntry{
		Collection:       resolved.Collection,
		Path:             displayDocumentPath(path, s.cfg.ContentDir),
		OriginalPath:     displayDocumentPath(originalPath, s.cfg.ContentDir),
		Reference:        ref,
		CurrentReference: currentRef,
		Name:             filepath.Base(path),
		PublicURL:        resolved.PublicURL,
		Kind:             string(resolved.Kind),
		Size:             info.Size(),
		State:            toLifecycleState(state),
		Timestamp:        timestamp,
		VersionComment:   versionComment,
		Actor:            actor,
		Metadata:         metadata,
	}, nil
}

func (s *Service) mediaMetadataHistoryEntry(path string) (types.MediaHistoryEntry, error) {
	info, err := s.fs.Stat(path)
	if err != nil {
		return types.MediaHistoryEntry{}, err
	}
	body, err := s.fs.ReadFile(path)
	if err != nil {
		return types.MediaHistoryEntry{}, err
	}
	var doc mediaMetadataDocument
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return types.MediaHistoryEntry{}, err
	}
	originalPath, state, ts, ok := lifecycle.ParsePathDetails(path)
	if !ok {
		originalPath = path
		state = lifecycle.StateCurrent
	}
	var timestamp *time.Time
	if !ts.IsZero() {
		timestamp = &ts
	}
	primaryOriginal := strings.TrimSuffix(originalPath, ".meta.yaml")
	_, currentRef, resolved, err := s.mediaHistoryReferenceInfo(primaryOriginal, primaryOriginal, lifecycle.StateCurrent)
	if err != nil {
		return types.MediaHistoryEntry{}, err
	}
	return types.MediaHistoryEntry{
		Collection:       resolved.Collection,
		Path:             displayDocumentPath(path, s.cfg.ContentDir),
		OriginalPath:     displayDocumentPath(originalPath, s.cfg.ContentDir),
		CurrentReference: currentRef,
		Name:             filepath.Base(primaryOriginal) + " metadata",
		Kind:             "metadata",
		Size:             info.Size(),
		State:            toLifecycleState(state),
		Timestamp:        timestamp,
		VersionComment:   strings.TrimSpace(doc.VersionComment),
		Actor:            strings.TrimSpace(doc.VersionActor),
		MetadataOnly:     true,
		Metadata:         normalizeMediaMetadata(doc.MediaMetadata),
	}, nil
}

func (s *Service) resolveDocumentLifecyclePath(path string) (string, string, lifecycle.State, error) {
	fullPath, err := s.resolveContentPathAllowDerived(path)
	if err != nil {
		return "", "", "", err
	}
	originalPath, state, ok := lifecycle.ParsePath(fullPath)
	if !ok {
		originalPath = fullPath
		state = lifecycle.StateCurrent
	}
	return fullPath, originalPath, state, nil
}

func (s *Service) resolveContentPathAllowDerived(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("source path is required")
	}

	contentRoot, err := filepath.Abs(s.cfg.ContentDir)
	if err != nil {
		return "", err
	}

	var full string
	if filepath.IsAbs(path) {
		full = filepath.Clean(path)
	} else {
		clean := filepath.Clean(path)
		contentDirSlash := filepath.ToSlash(s.cfg.ContentDir)
		contentBase := filepath.Base(s.cfg.ContentDir)
		cleanSlash := filepath.ToSlash(clean)
		switch {
		case strings.HasPrefix(cleanSlash, contentDirSlash+"/") || clean == s.cfg.ContentDir:
			full = clean
		case strings.HasPrefix(cleanSlash, contentBase+"/") || clean == contentBase:
			full = filepath.Join(filepath.Dir(s.cfg.ContentDir), clean)
		default:
			full = filepath.Join(s.cfg.ContentDir, clean)
		}
	}
	full, err = filepath.Abs(full)
	if err != nil {
		return "", err
	}

	rootWithSep := contentRoot + string(filepath.Separator)
	if full != contentRoot && !strings.HasPrefix(full, rootWithSep) {
		return "", fmt.Errorf("source path must be inside %s", s.cfg.ContentDir)
	}
	if filepath.Ext(full) != ".md" {
		return "", fmt.Errorf("source path must point to a markdown file")
	}
	if err := ensureNoSymlinkEscape(contentRoot, full); err != nil {
		return "", err
	}
	return full, nil
}

func (s *Service) resolveMediaLifecyclePath(path string) (string, string, lifecycle.State, error) {
	fullPath, err := s.resolveMediaPathAllowDerived(path)
	if err != nil {
		return "", "", "", err
	}
	originalPath, state, ok := lifecycle.ParsePath(fullPath)
	if !ok {
		originalPath = fullPath
		state = lifecycle.StateCurrent
	}
	return fullPath, originalPath, state, nil
}

func (s *Service) resolveMediaPathAllowDerived(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("media path is required")
	}

	absPath := path
	if !filepath.IsAbs(absPath) {
		clean := filepath.Clean(path)
		contentDirSlash := filepath.ToSlash(s.cfg.ContentDir)
		contentBase := filepath.Base(s.cfg.ContentDir)
		cleanSlash := filepath.ToSlash(clean)
		switch {
		case strings.HasPrefix(cleanSlash, contentDirSlash+"/") || clean == s.cfg.ContentDir:
			absPath = clean
		case strings.HasPrefix(cleanSlash, contentBase+"/") || clean == contentBase:
			absPath = filepath.Join(filepath.Dir(s.cfg.ContentDir), clean)
		default:
			absPath = filepath.Join(s.cfg.ContentDir, clean)
		}
	}
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return "", err
	}

	for _, collection := range []string{"images", "videos", "audio", "documents", "uploads", "assets"} {
		root, err := s.mediaRoot(collection)
		if err != nil {
			return "", err
		}
		root, err = filepath.Abs(root)
		if err != nil {
			return "", err
		}
		rootWithSep := root + string(filepath.Separator)
		if absPath == root || strings.HasPrefix(absPath, rootWithSep) {
			if err := ensureNoSymlinkEscape(root, absPath); err != nil {
				return "", err
			}
			return absPath, nil
		}
	}
	return "", fmt.Errorf("media path must be inside a media collection root")
}

func (s *Service) mediaReferenceInfoForPath(fullPath string) (string, string, media.Reference, error) {
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", "", media.Reference{}, err
	}
	for _, collection := range []string{"images", "videos", "audio", "documents", "uploads", "assets"} {
		root, err := s.mediaRoot(collection)
		if err != nil {
			return "", "", media.Reference{}, err
		}
		root, err = filepath.Abs(root)
		if err != nil {
			return "", "", media.Reference{}, err
		}
		rel, err := filepath.Rel(root, absFullPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}

		originalRel := filepath.ToSlash(rel)
		if original, _, ok := lifecycle.ParsePath(absFullPath); ok {
			if originalRelValue, err := filepath.Rel(root, original); err == nil {
				originalRel = filepath.ToSlash(originalRelValue)
			}
		}

		relSlash := filepath.ToSlash(rel)
		reference := media.ReferenceScheme + collection + "/" + relSlash
		currentReference := media.ReferenceScheme + collection + "/" + originalRel
		resolved := media.Reference{
			Collection: collection,
			Path:       relSlash,
			PublicURL:  "/" + collection + "/" + relSlash,
			Kind:       media.DetectKind(relSlash),
		}
		return reference, currentReference, resolved, nil
	}
	return "", "", media.Reference{}, fmt.Errorf("media path must stay inside a media collection root")
}

func (s *Service) mediaHistoryReferenceInfo(path, originalPath string, state lifecycle.State) (string, string, media.Reference, error) {
	_, currentReference, currentResolved, err := s.mediaReferenceInfoForPath(originalPath)
	if err != nil {
		return "", "", media.Reference{}, err
	}
	if state == lifecycle.StateCurrent {
		return currentReference, currentReference, currentResolved, nil
	}

	reference, _, derivedResolved, err := s.mediaReferenceInfoForPath(path)
	if err != nil {
		return currentReference, currentReference, currentResolved, nil
	}
	return reference, currentReference, derivedResolved, nil
}

func (s *Service) restoreMediaMetadata(oldPrimaryPath, restoredPrimaryPath string) error {
	sidecar := mediaMetadataPath(oldPrimaryPath)
	if _, err := s.fs.Stat(sidecar); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return s.fs.Rename(sidecar, mediaMetadataPath(restoredPrimaryPath))
}

func (s *Service) walkDir(root string, visit func(path string, info os.DirEntry) error) error {
	entries, err := s.fs.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		fullPath := filepath.Join(root, entry.Name())
		if entry.IsDir() {
			if err := visit(fullPath, entry); err != nil {
				return err
			}
			if err := s.walkDir(fullPath, visit); err != nil {
				return err
			}
			continue
		}
		if err := visit(fullPath, entry); err != nil {
			return err
		}
	}
	return nil
}

func sortDocumentHistoryEntries(entries []types.DocumentHistoryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].State != entries[j].State {
			if entries[i].State == types.LifecycleStateCurrent {
				return true
			}
			if entries[j].State == types.LifecycleStateCurrent {
				return false
			}
		}
		leftTS := time.Time{}
		rightTS := time.Time{}
		if entries[i].Timestamp != nil {
			leftTS = *entries[i].Timestamp
		}
		if entries[j].Timestamp != nil {
			rightTS = *entries[j].Timestamp
		}
		if !leftTS.Equal(rightTS) {
			return leftTS.After(rightTS)
		}
		return entries[i].Path < entries[j].Path
	})
}

func sortMediaHistoryEntries(entries []types.MediaHistoryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].State != entries[j].State {
			if entries[i].State == types.LifecycleStateCurrent {
				return true
			}
			if entries[j].State == types.LifecycleStateCurrent {
				return false
			}
		}
		leftTS := time.Time{}
		rightTS := time.Time{}
		if entries[i].Timestamp != nil {
			leftTS = *entries[i].Timestamp
		}
		if entries[j].Timestamp != nil {
			rightTS = *entries[j].Timestamp
		}
		if !leftTS.Equal(rightTS) {
			return leftTS.After(rightTS)
		}
		return entries[i].Path < entries[j].Path
	})
}

func toLifecycleState(state lifecycle.State) types.LifecycleState {
	switch state {
	case lifecycle.StateVersion:
		return types.LifecycleStateVersion
	case lifecycle.StateTrash:
		return types.LifecycleStateTrash
	default:
		return types.LifecycleStateCurrent
	}
}

func documentLangFromFrontMatter(fm *content.FrontMatter, fallback string) string {
	if fm == nil || fm.Params == nil {
		return fallback
	}
	if value, ok := fm.Params["lang"].(string); ok {
		return normalizeDocumentLang(value, fallback)
	}
	return fallback
}

func versionCommentFromFrontMatter(fm *content.FrontMatter) string {
	if fm == nil || fm.Params == nil {
		return ""
	}
	if value, ok := fm.Params["version_comment"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func versionActorFromFrontMatter(fm *content.FrontMatter) string {
	if fm == nil || fm.Params == nil {
		return ""
	}
	if value, ok := fm.Params["version_actor"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func buildUnifiedLineDiff(leftPath string, leftBody []byte, rightPath string, rightBody []byte) string {
	leftLines := splitLinesForDiff(leftBody)
	rightLines := splitLinesForDiff(rightBody)
	matches := buildLineLCS(leftLines, rightLines)

	var buf bytes.Buffer
	_, _ = fmt.Fprintf(&buf, "--- %s\n", filepath.ToSlash(leftPath))
	_, err := fmt.Fprintf(&buf, "+++ %s\n", filepath.ToSlash(rightPath))
	if err != nil {
		// TODO Handle this error at some point, even if redundant
	}
	for _, line := range matches {
		buf.WriteString(line.prefix)
		buf.WriteString(line.text)
		buf.WriteByte('\n')
	}
	return strings.TrimRight(buf.String(), "\n")
}

func (s *Service) loadMediaHistoryMetadata(path string) (types.MediaMetadata, string, string, error) {
	body, err := s.fs.ReadFile(mediaMetadataPath(path))
	if err != nil {
		if os.IsNotExist(err) {
			return types.MediaMetadata{}, "", "", nil
		}
		return types.MediaMetadata{}, "", "", err
	}
	var doc mediaMetadataDocument
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return types.MediaMetadata{}, "", "", err
	}
	return normalizeMediaMetadata(doc.MediaMetadata), strings.TrimSpace(doc.VersionComment), strings.TrimSpace(doc.VersionActor), nil
}

type diffLine struct {
	prefix string
	text   string
}

func buildLineLCS(left, right []string) []diffLine {
	dp := make([][]int, len(left)+1)
	for i := range dp {
		dp[i] = make([]int, len(right)+1)
	}
	for i := len(left) - 1; i >= 0; i-- {
		for j := len(right) - 1; j >= 0; j-- {
			if left[i] == right[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	out := make([]diffLine, 0, len(left)+len(right))
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		if left[i] == right[j] {
			out = append(out, diffLine{prefix: " ", text: left[i]})
			i++
			j++
			continue
		}
		if dp[i+1][j] >= dp[i][j+1] {
			out = append(out, diffLine{prefix: "-", text: left[i]})
			i++
		} else {
			out = append(out, diffLine{prefix: "+", text: right[j]})
			j++
		}
	}
	for i < len(left) {
		out = append(out, diffLine{prefix: "-", text: left[i]})
		i++
	}
	for j < len(right) {
		out = append(out, diffLine{prefix: "+", text: right[j]})
		j++
	}
	return out
}

func splitLinesForDiff(body []byte) []string {
	s := strings.ReplaceAll(string(body), "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func (s *Service) mediaUsage(reference string) ([]types.DocumentSummary, error) {
	graph, err := s.load(context.Background(), true)
	if err != nil {
		return nil, err
	}
	out := make([]types.DocumentSummary, 0)
	for _, doc := range graph.Documents {
		raw, err := s.fs.ReadFile(doc.SourcePath)
		if err != nil {
			return nil, err
		}
		if strings.Contains(string(raw), reference) {
			out = append(out, toSummary(doc))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].Lang != out[j].Lang {
			return out[i].Lang < out[j].Lang
		}
		return out[i].SourcePath < out[j].SourcePath
	})
	return out, nil
}
