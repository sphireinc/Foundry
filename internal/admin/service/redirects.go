package service

import (
	"context"
	"os"
	"path/filepath"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/redirects"
	"gopkg.in/yaml.v3"
)

func (s *Service) ListRedirects(ctx context.Context) (*types.RedirectListResponse, error) {
	_ = ctx
	path := redirects.Path(s.cfg)
	raw, err := s.fs.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &types.RedirectListResponse{Path: path, Redirects: []types.RedirectRule{}}, nil
		}
		return nil, err
	}
	store, err := redirects.Parse(raw)
	if err != nil {
		return nil, err
	}
	return &types.RedirectListResponse{
		Path:      path,
		Redirects: redirectRulesToAdmin(store.Redirects),
	}, nil
}

func (s *Service) SaveRedirects(ctx context.Context, input []types.RedirectRule) (*types.RedirectListResponse, error) {
	_ = ctx
	store := &redirects.Store{Redirects: make([]redirects.Rule, 0, len(input))}
	for _, rule := range input {
		store.Redirects = append(store.Redirects, redirects.Rule{
			From:          rule.From,
			To:            rule.To,
			Status:        rule.Status,
			Enabled:       rule.Enabled,
			PreserveQuery: rule.PreserveQuery,
			Note:          rule.Note,
		})
	}
	normalized, err := redirects.NormalizeStore(store)
	if err != nil {
		return nil, err
	}
	body, err := yaml.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	path := redirects.Path(s.cfg)
	if err := s.fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := s.fs.WriteFile(path, body, 0o644); err != nil {
		return nil, err
	}
	s.invalidateGraphCache()
	return &types.RedirectListResponse{
		Path:      path,
		Redirects: redirectRulesToAdmin(normalized.Redirects),
	}, nil
}

func redirectRulesToAdmin(in []redirects.Rule) []types.RedirectRule {
	out := make([]types.RedirectRule, 0, len(in))
	for _, rule := range in {
		out = append(out, types.RedirectRule{
			From:          rule.From,
			To:            rule.To,
			Status:        rule.Status,
			Enabled:       rule.Enabled,
			PreserveQuery: rule.PreserveQuery,
			Note:          rule.Note,
		})
	}
	return out
}
