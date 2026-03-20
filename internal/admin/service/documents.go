package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/i18n"
	"github.com/sphireinc/foundry/internal/lifecycle"
	"github.com/sphireinc/foundry/internal/markup"
	"github.com/sphireinc/foundry/internal/safepath"
	"gopkg.in/yaml.v3"
)

func (s *Service) ListDocuments(ctx context.Context, opts types.DocumentListOptions) ([]types.DocumentSummary, error) {
	graph, err := s.load(ctx, opts.IncludeDrafts)
	if err != nil {
		return nil, err
	}

	rows := make([]types.DocumentSummary, 0, len(graph.Documents))
	query := strings.ToLower(strings.TrimSpace(opts.Query))

	for _, doc := range graph.Documents {
		if !opts.IncludeDrafts && doc.Draft {
			continue
		}
		if opts.Type != "" && doc.Type != opts.Type {
			continue
		}
		if opts.Lang != "" && doc.Lang != opts.Lang {
			continue
		}
		if query != "" && !matchesDocumentQuery(doc, query) {
			continue
		}

		rows = append(rows, toSummary(doc))
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Type != rows[j].Type {
			return rows[i].Type < rows[j].Type
		}
		if rows[i].Lang != rows[j].Lang {
			return rows[i].Lang < rows[j].Lang
		}
		if rows[i].URL != rows[j].URL {
			return rows[i].URL < rows[j].URL
		}
		return rows[i].SourcePath < rows[j].SourcePath
	})

	return rows, nil
}

func (s *Service) GetDocument(ctx context.Context, idOrPath string, includeDrafts bool) (*types.DocumentDetail, error) {
	graph, err := s.load(ctx, includeDrafts)
	if err != nil {
		return nil, err
	}

	idOrPath = strings.TrimSpace(idOrPath)
	if idOrPath == "" {
		return nil, fmt.Errorf("document id or path is required")
	}

	for _, doc := range graph.Documents {
		if doc.ID == idOrPath || doc.SourcePath == idOrPath || displayDocumentPath(doc.SourcePath, s.cfg.ContentDir) == idOrPath || doc.URL == idOrPath {
			detail, err := s.toDetail(doc)
			if err != nil {
				return nil, err
			}
			return &detail, nil
		}
	}

	return nil, fmt.Errorf("document not found: %s", idOrPath)
}

func (s *Service) SaveDocument(ctx context.Context, req types.DocumentSaveRequest) (*types.DocumentSaveResponse, error) {
	_ = ctx

	sourcePath, err := s.resolveContentPath(req.SourcePath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Raw) == "" {
		return nil, fmt.Errorf("raw document body is required")
	}

	created := false
	now := time.Now()
	if _, err := s.fs.Stat(sourcePath); err != nil {
		created = true
	} else {
		if strings.TrimSpace(req.VersionComment) != "" || strings.TrimSpace(req.Actor) != "" {
			if err := s.snapshotDocumentVersion(sourcePath, now, req.VersionComment, req.Actor); err != nil {
				return nil, err
			}
		} else if err := s.versionFile(sourcePath, now); err != nil {
			return nil, err
		}
	}

	if err := s.fs.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		return nil, err
	}
	if err := s.fs.WriteFile(sourcePath, []byte(req.Raw), 0o644); err != nil {
		return nil, err
	}
	s.invalidateGraphCache()

	return &types.DocumentSaveResponse{
		SourcePath: displayDocumentPath(sourcePath, s.cfg.ContentDir),
		Size:       int64(len(req.Raw)),
		Created:    created,
	}, nil
}

func (s *Service) CreateDocument(ctx context.Context, req types.DocumentCreateRequest) (*types.DocumentCreateResponse, error) {
	_ = ctx

	kind := normalizeDocumentKind(req.Kind)
	if kind == "" {
		return nil, fmt.Errorf("document kind must be page or post")
	}

	slug := sanitizeDocumentSlug(req.Slug)
	if slug == "" {
		return nil, fmt.Errorf("document slug is required")
	}

	body, err := content.BuildNewContentWithOptions(s.cfg, content.NewContentOptions{
		Kind:      kind,
		Slug:      slug,
		Archetype: strings.TrimSpace(req.Archetype),
		Lang:      strings.TrimSpace(req.Lang),
	})
	if err != nil {
		return nil, err
	}

	lang := normalizeDocumentLang(req.Lang, s.cfg.DefaultLang)
	relPath := s.newDocumentRelativePath(kind, lang, slug)
	sourcePath, err := s.resolveContentPath(relPath)
	if err != nil {
		return nil, err
	}
	if _, err := s.fs.Stat(sourcePath); err == nil {
		return nil, fmt.Errorf("document already exists: %s", filepath.ToSlash(relPath))
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if err := s.fs.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		return nil, err
	}
	if err := s.fs.WriteFile(sourcePath, []byte(body), 0o644); err != nil {
		return nil, err
	}
	s.invalidateGraphCache()

	return &types.DocumentCreateResponse{
		Kind:       kind,
		Slug:       slug,
		Lang:       lang,
		Archetype:  strings.TrimSpace(req.Archetype),
		SourcePath: displayDocumentPath(sourcePath, s.cfg.ContentDir),
		Created:    true,
		Raw:        body,
	}, nil
}

func (s *Service) UpdateDocumentStatus(ctx context.Context, req types.DocumentStatusRequest) (*types.DocumentStatusResponse, error) {
	_ = ctx

	sourcePath, err := s.resolveContentPath(req.SourcePath)
	if err != nil {
		return nil, err
	}
	raw, err := s.fs.ReadFile(sourcePath)
	if err != nil {
		return nil, err
	}

	fm, body, err := content.ParseDocument(raw)
	if err != nil {
		return nil, err
	}

	status := normalizeDocumentStatus(req.Status)
	if status == "" {
		return nil, fmt.Errorf("document status must be published, draft, or archived")
	}

	if fm.Params == nil {
		fm.Params = make(map[string]any)
	}
	switch status {
	case "published":
		fm.Draft = false
		delete(fm.Params, "archived")
	case "draft":
		fm.Draft = true
		delete(fm.Params, "archived")
	case "archived":
		fm.Draft = true
		fm.Params["archived"] = true
	}
	now := time.Now().UTC()
	fm.UpdatedAt = &now

	rendered, err := marshalDocument(fm, body)
	if err != nil {
		return nil, err
	}
	if err := s.fs.WriteFile(sourcePath, rendered, 0o644); err != nil {
		return nil, err
	}
	s.invalidateGraphCache()

	return &types.DocumentStatusResponse{
		SourcePath: displayDocumentPath(sourcePath, s.cfg.ContentDir),
		Status:     status,
		Draft:      fm.Draft,
		Archived:   documentArchivedFromParams(fm.Params),
	}, nil
}

func (s *Service) DeleteDocument(ctx context.Context, req types.DocumentDeleteRequest) (*types.DocumentDeleteResponse, error) {
	_ = ctx

	sourcePath, err := s.resolveContentPath(req.SourcePath)
	if err != nil {
		return nil, err
	}
	if _, err := s.fs.Stat(sourcePath); err != nil {
		return nil, err
	}

	trashPath, err := s.trashFile(sourcePath, time.Now())
	if err != nil {
		return nil, err
	}
	s.invalidateGraphCache()

	return &types.DocumentDeleteResponse{
		SourcePath: displayDocumentPath(sourcePath, s.cfg.ContentDir),
		TrashPath:  displayDocumentPath(trashPath, s.cfg.ContentDir),
		Operation:  "soft_delete",
	}, nil
}

func (s *Service) PreviewDocument(ctx context.Context, req types.DocumentPreviewRequest) (*types.DocumentPreviewResponse, error) {
	_ = ctx

	raw := req.Raw
	if strings.TrimSpace(raw) == "" && strings.TrimSpace(req.SourcePath) != "" {
		sourcePath, err := s.resolveContentPath(req.SourcePath)
		if err != nil {
			return nil, err
		}
		b, err := s.fs.ReadFile(sourcePath)
		if err != nil {
			return nil, err
		}
		raw = string(b)
	}

	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("preview requires raw content or source_path")
	}

	fm, body, err := content.ParseDocument([]byte(raw))
	if err != nil {
		return nil, err
	}

	htmlBody, err := markup.MarkdownToHTML(body, s.cfg.Security.AllowUnsafeHTML)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(fm.Title)
	if title == "" {
		title = strings.TrimSpace(fm.Slug)
	}

	return &types.DocumentPreviewResponse{
		Title:     title,
		Slug:      fm.Slug,
		Layout:    fm.Layout,
		Summary:   fm.Summary,
		Draft:     fm.Draft,
		Date:      fm.Date,
		UpdatedAt: fm.UpdatedAt,
		HTML:      string(htmlBody),
		WordCount: countWords(body),
	}, nil
}

func matchesDocumentQuery(doc *content.Document, query string) bool {
	candidates := []string{
		doc.ID,
		doc.Title,
		doc.Slug,
		doc.URL,
		doc.SourcePath,
		doc.Type,
		doc.Lang,
		doc.Summary,
	}

	for _, c := range candidates {
		if strings.Contains(strings.ToLower(c), query) {
			return true
		}
	}
	return false
}

func toSummary(doc *content.Document) types.DocumentSummary {
	return types.DocumentSummary{
		ID:         doc.ID,
		Type:       doc.Type,
		Lang:       doc.Lang,
		Title:      doc.Title,
		Slug:       doc.Slug,
		URL:        doc.URL,
		Layout:     doc.Layout,
		SourcePath: displayDocumentPath(doc.SourcePath, ""),
		Summary:    doc.Summary,
		Draft:      doc.Draft,
		Archived:   documentArchivedFromParams(doc.Params),
		Date:       doc.Date,
		UpdatedAt:  doc.UpdatedAt,
		Taxonomies: doc.Taxonomies,
	}
}

func (s *Service) toDetail(doc *content.Document) (types.DocumentDetail, error) {
	raw, err := s.fs.ReadFile(doc.SourcePath)
	if err != nil {
		return types.DocumentDetail{}, err
	}
	return types.DocumentDetail{
		DocumentSummary: toSummary(doc),
		RawBody:         string(raw),
		HTMLBody:        string(doc.HTMLBody),
		Params:          doc.Params,
		Fields:          doc.Fields,
	}, nil
}

func countWords(s string) int {
	return len(strings.Fields(s))
}

func documentArchivedFromParams(params map[string]any) bool {
	if len(params) == 0 {
		return false
	}
	value, ok := params["archived"]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func normalizeDocumentKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "page", "post":
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return ""
	}
}

func normalizeDocumentStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "published", "draft", "archived":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return ""
	}
}

func sanitizeDocumentSlug(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ' || r == '/':
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func (s *Service) newDocumentRelativePath(kind, lang, slug string) string {
	root := s.cfg.Content.PagesDir
	if kind == "post" {
		root = s.cfg.Content.PostsDir
	}
	if lang != "" && lang != s.cfg.DefaultLang {
		return filepath.ToSlash(filepath.Join(root, lang, slug+".md"))
	}
	return filepath.ToSlash(filepath.Join(root, slug+".md"))
}

func normalizeDocumentLang(lang, fallback string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return fallback
	}
	lang = i18n.NormalizeTag(lang)
	if !i18n.IsValidTag(lang) {
		return fallback
	}
	return lang
}

func displayDocumentPath(path, contentRoot string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	if contentRoot != "" {
		root, err := filepath.Abs(strings.TrimSpace(contentRoot))
		if err == nil && root != "" {
			root = filepath.ToSlash(root)
			if rel, err := filepath.Rel(root, filepath.FromSlash(path)); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
				return filepath.ToSlash(filepath.Join(filepath.Base(root), rel))
			}
		}
	}
	if idx := strings.Index(path, "/content/"); idx >= 0 {
		return path[idx+1:]
	}
	if strings.HasPrefix(path, "content/") {
		return path
	}
	return path
}

func marshalDocument(fm *content.FrontMatter, body string) ([]byte, error) {
	payload, err := yaml.Marshal(fm)
	if err != nil {
		return nil, err
	}
	body = strings.TrimLeft(body, "\n")
	rendered := "---\n" + string(payload) + "---\n\n" + body
	return []byte(rendered), nil
}

func (s *Service) resolveContentPath(path string) (string, error) {
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
	if lifecycle.IsDerivedPath(full) {
		return "", fmt.Errorf("source path must point to a current markdown file")
	}
	if err := ensureNoSymlinkEscape(contentRoot, full); err != nil {
		return "", err
	}

	return full, nil
}

func ensureNoSymlinkEscape(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("source path must be inside %s", root)
	}

	current := rootAbs
	if rel == "." {
		return nil
	}

	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)

		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				ok, err := safepath.IsWithinRoot(rootAbs, current)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("source path must be inside %s", root)
				}
				continue
			}
			return err
		}

		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			return err
		}
		ok, err := safepath.IsWithinRoot(rootAbs, resolved)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("source path must stay inside %s after resolving symlinks", root)
		}
	}

	return nil
}
