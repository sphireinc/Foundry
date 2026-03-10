package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/markup"
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
		if doc.ID == idOrPath || doc.SourcePath == idOrPath || doc.URL == idOrPath {
			detail := toDetail(doc)
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
	if _, err := s.fs.Stat(sourcePath); err != nil {
		created = true
	}

	if err := s.fs.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		return nil, err
	}
	if err := s.fs.WriteFile(sourcePath, []byte(req.Raw), 0o644); err != nil {
		return nil, err
	}

	return &types.DocumentSaveResponse{
		SourcePath: filepath.ToSlash(sourcePath),
		Size:       int64(len(req.Raw)),
		Created:    created,
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

	htmlBody, err := markup.MarkdownToHTML(body)
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
		SourcePath: doc.SourcePath,
		Summary:    doc.Summary,
		Draft:      doc.Draft,
		Date:       doc.Date,
		UpdatedAt:  doc.UpdatedAt,
		Taxonomies: doc.Taxonomies,
	}
}

func toDetail(doc *content.Document) types.DocumentDetail {
	return types.DocumentDetail{
		DocumentSummary: toSummary(doc),
		RawBody:         doc.RawBody,
		HTMLBody:        string(doc.HTMLBody),
		Params:          doc.Params,
		Fields:          doc.Fields,
	}
}

func countWords(s string) int {
	return len(strings.Fields(s))
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
		if strings.HasPrefix(filepath.ToSlash(clean), filepath.ToSlash(s.cfg.ContentDir)+"/") || clean == s.cfg.ContentDir {
			full = clean
		} else {
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

	return full, nil
}
