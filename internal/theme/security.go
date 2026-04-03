package theme

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

func normalizeThemeSecurity(sec *ThemeSecurity) {
	if sec == nil {
		return
	}
	sec.ExternalAssets.Scripts = normalizeSecurityList(sec.ExternalAssets.Scripts)
	sec.ExternalAssets.Styles = normalizeSecurityList(sec.ExternalAssets.Styles)
	sec.ExternalAssets.Fonts = normalizeSecurityList(sec.ExternalAssets.Fonts)
	sec.ExternalAssets.Images = normalizeSecurityList(sec.ExternalAssets.Images)
	sec.ExternalAssets.Media = normalizeSecurityList(sec.ExternalAssets.Media)
	sec.FrontendRequests.Origins = normalizeSecurityList(sec.FrontendRequests.Origins)
	sec.FrontendRequests.Methods = normalizeHTTPMethods(sec.FrontendRequests.Methods)
	if len(sec.FrontendRequests.Methods) == 0 {
		sec.FrontendRequests.Methods = []string{"GET"}
	}
	if sec.TemplateContext.AllowSiteParams == nil {
		sec.TemplateContext.AllowSiteParams = boolPtr(true)
	}
	if sec.TemplateContext.AllowContentFields == nil {
		sec.TemplateContext.AllowContentFields = boolPtr(true)
	}
	if sec.TemplateContext.AllowSharedFields == nil {
		sec.TemplateContext.AllowSharedFields = boolPtr(true)
	}
	if sec.TemplateContext.AllowRuntimeState == nil {
		sec.TemplateContext.AllowRuntimeState = boolPtr(false)
	}
	if sec.TemplateContext.AllowAdminState == nil {
		sec.TemplateContext.AllowAdminState = boolPtr(false)
	}
	if sec.TemplateContext.AllowRawConfig == nil {
		sec.TemplateContext.AllowRawConfig = boolPtr(false)
	}
}

func normalizeSecurityList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if u, err := url.Parse(item); err == nil && u.Scheme != "" && u.Host != "" {
			item = strings.TrimRight(u.Scheme+"://"+u.Host, "/")
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func normalizeHTTPMethods(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.ToUpper(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func themeExternalAssetAllowlist(sec ThemeSecurity) []string {
	out := append([]string{}, sec.ExternalAssets.Scripts...)
	out = append(out, sec.ExternalAssets.Styles...)
	out = append(out, sec.ExternalAssets.Fonts...)
	out = append(out, sec.ExternalAssets.Images...)
	out = append(out, sec.ExternalAssets.Media...)
	return normalizeSecurityList(out)
}

func URLAllowedByPatterns(raw string, patterns []string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	origin := strings.TrimRight(u.Scheme+"://"+u.Host, "/")
	host := strings.ToLower(u.Hostname())
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if strings.Contains(pattern, "://") {
			p, err := url.Parse(pattern)
			if err == nil && p.Scheme != "" && p.Host != "" {
				if origin == strings.TrimRight(p.Scheme+"://"+p.Host, "/") || strings.HasPrefix(raw, pattern) {
					return true
				}
			}
			if strings.HasPrefix(raw, pattern) {
				return true
			}
			continue
		}
		patternHost := strings.ToLower(strings.TrimPrefix(pattern, "*."))
		if host == patternHost || strings.HasSuffix(host, "."+patternHost) {
			return true
		}
	}
	return false
}

func ContentSecurityPolicy(manifest *Manifest) string {
	sec := ThemeSecurity{}
	if manifest != nil {
		sec = manifest.Security
	}
	normalizeThemeSecurity(&sec)

	directives := [][]string{
		{"default-src", "'self'"},
		{"base-uri", "'self'"},
		{"object-src", "'none'"},
		{"frame-ancestors", "'self'"},
	}

	directives = append(directives, append([]string{"script-src", "'self'", "'unsafe-inline'"}, sec.ExternalAssets.Scripts...))
	directives = append(directives, append([]string{"style-src", "'self'", "'unsafe-inline'"}, sec.ExternalAssets.Styles...))
	directives = append(directives, append([]string{"font-src", "'self'", "data:"}, sec.ExternalAssets.Fonts...))
	directives = append(directives, append([]string{"img-src", "'self'", "data:", "blob:"}, sec.ExternalAssets.Images...))
	directives = append(directives, append([]string{"media-src", "'self'", "data:", "blob:"}, sec.ExternalAssets.Media...))

	connect := []string{"connect-src", "'self'"}
	if sec.FrontendRequests.Allowed {
		connect = append(connect, sec.FrontendRequests.Origins...)
	}
	directives = append(directives, connect)

	parts := make([]string, 0, len(directives))
	for _, directive := range directives {
		parts = append(parts, strings.Join(uniqueDirectiveValues(directive), " "))
	}
	return strings.Join(parts, "; ")
}

func uniqueDirectiveValues(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s ThemeSecurity) Summary() []string {
	normalizeThemeSecurity(&s)
	out := []string{}
	if s.ExternalAssets.Allowed && len(themeExternalAssetAllowlist(s)) > 0 {
		out = append(out, fmt.Sprintf("Remote assets: %d declared source(s)", len(themeExternalAssetAllowlist(s))))
	} else {
		out = append(out, "Remote assets blocked")
	}
	if s.FrontendRequests.Allowed && len(s.FrontendRequests.Origins) > 0 {
		out = append(out, fmt.Sprintf("Frontend requests: %s", strings.Join(s.FrontendRequests.Origins, ", ")))
	} else {
		out = append(out, "Frontend requests blocked")
	}
	if !themeBoolValue(s.TemplateContext.AllowRawConfig) {
		out = append(out, "Raw config hidden from templates")
	}
	if !themeBoolValue(s.TemplateContext.AllowAdminState) {
		out = append(out, "Admin state hidden from templates")
	}
	return out
}

func themeBoolValue(v *bool) bool {
	return v != nil && *v
}

func boolPtr(v bool) *bool {
	return &v
}
