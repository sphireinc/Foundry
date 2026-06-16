package redirects

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
	"gopkg.in/yaml.v3"
)

const FileName = "redirects.yaml"

type Store struct {
	Redirects []Rule `yaml:"redirects" json:"redirects"`
}

type Rule struct {
	From          string `yaml:"from" json:"from"`
	To            string `yaml:"to" json:"to"`
	Status        int    `yaml:"status,omitempty" json:"status"`
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	PreserveQuery bool   `yaml:"preserve_query,omitempty" json:"preserve_query"`
	Note          string `yaml:"note,omitempty" json:"note,omitempty"`
}

type fileStore struct {
	Redirects []fileRule `yaml:"redirects"`
}

type fileRule struct {
	From          string `yaml:"from"`
	To            string `yaml:"to"`
	Status        int    `yaml:"status,omitempty"`
	Enabled       *bool  `yaml:"enabled,omitempty"`
	PreserveQuery bool   `yaml:"preserve_query,omitempty"`
	Note          string `yaml:"note,omitempty"`
}

func Path(cfg *config.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.DataDir) == "" {
		return filepath.Join("data", FileName)
	}
	return filepath.Join(cfg.DataDir, FileName)
}

func Load(cfg *config.Config) (*Store, error) {
	path := Path(cfg)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{Redirects: []Rule{}}, nil
		}
		return nil, err
	}
	return Parse(b)
}

func Parse(b []byte) (*Store, error) {
	var wrapped fileStore
	if err := yaml.Unmarshal(b, &wrapped); err != nil {
		return nil, err
	}
	if len(wrapped.Redirects) == 0 {
		var list []fileRule
		if err := yaml.Unmarshal(b, &list); err == nil && len(list) > 0 {
			wrapped.Redirects = list
		}
	}
	store := &Store{Redirects: make([]Rule, 0, len(wrapped.Redirects))}
	for _, in := range wrapped.Redirects {
		enabled := true
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		store.Redirects = append(store.Redirects, Rule{
			From:          in.From,
			To:            in.To,
			Status:        in.Status,
			Enabled:       enabled,
			PreserveQuery: in.PreserveQuery,
			Note:          in.Note,
		})
	}
	return NormalizeStore(store)
}

func NormalizeStore(store *Store) (*Store, error) {
	if store == nil {
		store = &Store{}
	}
	out := &Store{Redirects: make([]Rule, 0, len(store.Redirects))}
	seen := make(map[string]struct{}, len(store.Redirects))
	for i, in := range store.Redirects {
		rule, err := NormalizeRule(in)
		if err != nil {
			return nil, fmt.Errorf("redirect %d: %w", i+1, err)
		}
		if _, ok := seen[rule.From]; ok {
			return nil, fmt.Errorf("redirect %d: duplicate source %q", i+1, rule.From)
		}
		seen[rule.From] = struct{}{}
		out.Redirects = append(out.Redirects, rule)
	}
	sort.SliceStable(out.Redirects, func(i, j int) bool {
		return out.Redirects[i].From < out.Redirects[j].From
	})
	return out, nil
}

func NormalizeRule(in Rule) (Rule, error) {
	from, err := NormalizeSourcePath(in.From)
	if err != nil {
		return Rule{}, fmt.Errorf("from: %w", err)
	}
	to, err := NormalizeTarget(in.To)
	if err != nil {
		return Rule{}, fmt.Errorf("to: %w", err)
	}
	status := in.Status
	if status == 0 {
		status = 301
	}
	if !validStatus(status) {
		return Rule{}, fmt.Errorf("status must be one of 301, 302, 307, or 308")
	}
	if from == normalizeComparableTarget(to) {
		return Rule{}, fmt.Errorf("source and target must differ")
	}
	return Rule{
		From:          from,
		To:            to,
		Status:        status,
		Enabled:       in.Enabled,
		PreserveQuery: in.PreserveQuery,
		Note:          strings.TrimSpace(in.Note),
	}, nil
}

func NormalizeSourcePath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.Contains(value, "://") {
		return "", fmt.Errorf("must be a site-relative path")
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("must not include query strings or fragments")
	}
	path := u.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return normalizePath(path), nil
}

func NormalizeTarget(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("target is required")
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if u.Scheme != "" {
		if u.Scheme != "http" && u.Scheme != "https" {
			return "", fmt.Errorf("external targets must use http or https")
		}
		if u.Host == "" {
			return "", fmt.Errorf("external targets must include a host")
		}
		return value, nil
	}
	if strings.HasPrefix(value, "//") {
		return "", fmt.Errorf("protocol-relative targets are not allowed")
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	pathValue := u.Path
	if !strings.HasPrefix(pathValue, "/") {
		pathValue = "/" + pathValue
	}
	path := normalizePath(pathValue)
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		path += "#" + u.Fragment
	}
	return path, nil
}

func Lookup(rules []Rule, requestPath string) (Rule, bool) {
	source, err := NormalizeSourcePath(requestPath)
	if err != nil {
		return Rule{}, false
	}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.From == source {
			return rule, true
		}
	}
	return Rule{}, false
}

func TargetWithQuery(rule Rule, rawQuery string) string {
	target := rule.To
	if !rule.PreserveQuery || strings.TrimSpace(rawQuery) == "" || strings.Contains(target, "?") {
		return target
	}
	if before, after, ok := strings.Cut(target, "#"); ok {
		return before + "?" + rawQuery + "#" + after
	}
	return target + "?" + rawQuery
}

func validStatus(status int) bool {
	switch status {
	case 301, 302, 307, 308:
		return true
	default:
		return false
	}
}

func normalizePath(path string) string {
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if path == "" {
		return "/"
	}
	path = strings.ReplaceAll(path, "\\", "/")
	path = filepath.ToSlash(filepath.Clean(path))
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path != "/" && !strings.Contains(filepath.Base(path), ".") && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func normalizeComparableTarget(target string) string {
	if strings.Contains(target, "://") {
		return target
	}
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	return normalizePath(u.Path)
}
