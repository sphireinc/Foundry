package aiwriter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/safepath"
)

const (
	pluginName            = "aiwriter"
	defaultTimeoutSeconds = 60
	defaultTemperature    = 0.7
	defaultMaxTokens      = 2400
	defaultOpenAIModel    = "gpt-4o-mini"
	defaultAnthropicModel = "claude-3-5-haiku-latest"
	defaultGeminiModel    = "gemini-1.5-flash"

	defaultOpenAIEndpoint = "https://api.openai.com/{version}/responses"
	defaultOpenAIVersion  = "v1"

	defaultAnthropicURL     = "https://api.anthropic.com/{version}/messages"
	defaultAnthropicVersion = "v1"

	defaultGeminiEndpoint = "https://generativelanguage.googleapis.com/{version}/models/%s:generateContent"
	defaultGeminiVersion  = "v1beta"

	defaultGeneratedStatus  = "draft"
	adminGenerateCapability = "documents.create"
)

var frontmatterValuePattern = regexp.MustCompile(`(?m)^([A-Za-z0-9_-]+):\s*"?([^"\n]+)"?\s*$`)

type Plugin struct {
	mu       sync.RWMutex
	cfg      *config.Config
	settings Settings
	client   *http.Client
	usage    UsageStats
}

type Settings struct {
	Provider        string  `json:"provider"`
	APIKey          string  `json:"-"`
	APIKeyEnv       string  `json:"api_key_env"`
	Model           string  `json:"model"`
	Endpoint        string  `json:"endpoint,omitempty"`
	EndpointVersion string  `json:"endpoint_version,omitempty"`
	SystemPrompt    string  `json:"system_prompt,omitempty"`
	DefaultStatus   string  `json:"default_status"`
	DefaultAuthor   string  `json:"default_author,omitempty"`
	DefaultLang     string  `json:"default_lang,omitempty"`
	Temperature     float64 `json:"temperature"`
	MaxTokens       int     `json:"max_tokens"`
	TimeoutSeconds  int     `json:"timeout_seconds"`
}

type GenerateRequest struct {
	Prompt     string   `json:"prompt"`
	Title      string   `json:"title,omitempty"`
	Slug       string   `json:"slug,omitempty"`
	Status     string   `json:"status,omitempty"`
	Author     string   `json:"author,omitempty"`
	Lang       string   `json:"lang,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

type GenerateResponse struct {
	Path     string     `json:"path"`
	Title    string     `json:"title"`
	Slug     string     `json:"slug"`
	Status   string     `json:"status"`
	Provider string     `json:"provider"`
	Model    string     `json:"model"`
	Markdown string     `json:"markdown"`
	Usage    TokenUsage `json:"usage"`
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type UsageStats struct {
	AIWrittenPosts          int        `json:"ai_written_posts"`
	GenerationsThisRun      int        `json:"generations_this_run"`
	PromptTokensThisRun     int        `json:"prompt_tokens_this_run"`
	CompletionTokensThisRun int        `json:"completion_tokens_this_run"`
	TotalTokensThisRun      int        `json:"total_tokens_this_run"`
	LastRequest             TokenUsage `json:"last_request"`
}

type settingsResponse struct {
	Settings
	Configured bool       `json:"configured"`
	KeySource  string     `json:"key_source"`
	Usage      UsageStats `json:"usage"`
}

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Name() string {
	return pluginName
}

func (p *Plugin) OnConfigLoaded(cfg *config.Config) error {
	settings := settingsFromConfig(cfg)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = cfg
	p.settings = settings
	p.client = &http.Client{Timeout: time.Duration(settings.TimeoutSeconds) * time.Second}
	return nil
}

func (p *Plugin) RegisterRoutes(mux *http.ServeMux) {
	cfg, _, _ := p.snapshot()
	if mux == nil || cfg == nil {
		return
	}

	auth := adminauth.New(cfg)
	base := strings.TrimRight(cfg.AdminPath(), "/") + "/plugin-api/aiwriter"
	mux.Handle(base+"/settings", auth.WrapCapability(http.HandlerFunc(p.handleSettings), adminGenerateCapability))
	mux.Handle(base+"/generate", auth.WrapCapability(http.HandlerFunc(p.handleGenerate), adminGenerateCapability))
}

func (p *Plugin) handleSettings(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg, settings, _ := p.snapshot()
	_, source := settings.resolveAPIKey()
	usage := p.usageSnapshot()
	usage.AIWrittenPosts = countAIWrittenPosts(cfg)
	writeJSON(w, http.StatusOK, settingsResponse{
		Settings:   settings.redacted(),
		Configured: source != "missing",
		KeySource:  source,
		Usage:      usage,
	})
}

func (p *Plugin) handleGenerate(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer req.Body.Close()

	var body GenerateRequest
	dec := json.NewDecoder(io.LimitReader(req.Body, 128*1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	resp, err := p.GenerateAndWrite(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (p *Plugin) GenerateAndWrite(ctx context.Context, body GenerateRequest) (*GenerateResponse, error) {
	cfg, settings, client := p.snapshot()
	if cfg == nil {
		return nil, fmt.Errorf("ai writer plugin is not configured")
	}
	if strings.TrimSpace(body.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if client == nil {
		client = http.DefaultClient
	}

	prompt := buildPrompt(settings, body)
	markdown, usage, err := requestCompletion(ctx, client, settings, prompt)
	if err != nil {
		return nil, err
	}
	p.recordUsage(usage)
	markdown = normalizeMarkdown(markdown)
	markdown, title, slug, status := completeMarkdownDocument(markdown, settings, body)

	path, err := writePost(cfg, markdown)
	if err != nil {
		return nil, err
	}

	return &GenerateResponse{
		Path:     filepath.ToSlash(path),
		Title:    title,
		Slug:     slug,
		Status:   status,
		Provider: settings.Provider,
		Model:    settings.Model,
		Markdown: markdown,
		Usage:    usage,
	}, nil
}

func (p *Plugin) snapshot() (*config.Config, Settings, *http.Client) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cfg, p.settings, p.client
}

func (p *Plugin) usageSnapshot() UsageStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.usage
}

func (p *Plugin) recordUsage(usage TokenUsage) {
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usage.GenerationsThisRun++
	p.usage.PromptTokensThisRun += usage.PromptTokens
	p.usage.CompletionTokensThisRun += usage.CompletionTokens
	p.usage.TotalTokensThisRun += usage.TotalTokens
	p.usage.LastRequest = usage
}

func settingsFromConfig(cfg *config.Config) Settings {
	defaultLang := ""
	if cfg != nil {
		defaultLang = strings.TrimSpace(cfg.DefaultLang)
	}
	settings := Settings{
		Provider:        "openai",
		APIKeyEnv:       "OPENAI_API_KEY",
		Model:           defaultOpenAIModel,
		EndpointVersion: defaultVersion("openai"),
		Endpoint:        defaultEndpoint("openai", defaultOpenAIModel, ""),
		DefaultStatus:   defaultGeneratedStatus,
		DefaultLang:     defaultLang,
		Temperature:     defaultTemperature,
		MaxTokens:       defaultMaxTokens,
		TimeoutSeconds:  defaultTimeoutSeconds,
	}
	if cfg == nil || cfg.Params == nil {
		return settings
	}

	raw, _ := cfg.Params["ai_writer"]
	values, ok := raw.(map[string]any)
	if !ok {
		if typed, ok := raw.(map[any]any); ok {
			values = make(map[string]any, len(typed))
			for k, v := range typed {
				values[fmt.Sprint(k)] = v
			}
		}
	}
	if len(values) == 0 {
		return settings
	}

	settings.Provider = lowerString(settingString(values, "provider", settings.Provider))
	settings.APIKey = settingString(values, "api_key", "")
	settings.APIKeyEnv = settingString(values, "api_key_env", defaultAPIKeyEnv(settings.Provider))
	if settings.APIKey == "" && settings.APIKeyEnv != "" && !isValidEnvVarName(settings.APIKeyEnv) {
		settings.APIKey = settings.APIKeyEnv
		settings.APIKeyEnv = defaultAPIKeyEnv(settings.Provider)
	}
	settings.Model = settingString(values, "model", defaultModel(settings.Provider))
	settings.EndpointVersion = settingString(values, "endpoint_version", defaultVersion(settings.Provider))
	settings.Endpoint = resolveEndpoint(
		settings.Provider,
		settings.Model,
		settingString(values, "endpoint", ""),
		settings.EndpointVersion,
	)
	settings.SystemPrompt = settingString(values, "system_prompt", "")
	settings.DefaultStatus = lowerString(settingString(values, "default_status", settings.DefaultStatus))
	settings.DefaultAuthor = settingString(values, "default_author", "")
	settings.DefaultLang = settingString(values, "default_lang", settings.DefaultLang)
	settings.Temperature = settingFloat(values, "temperature", settings.Temperature)
	settings.MaxTokens = settingInt(values, "max_tokens", settings.MaxTokens)
	settings.TimeoutSeconds = settingInt(values, "timeout_seconds", settings.TimeoutSeconds)

	if settings.TimeoutSeconds <= 0 {
		settings.TimeoutSeconds = defaultTimeoutSeconds
	}
	if settings.MaxTokens <= 0 {
		settings.MaxTokens = defaultMaxTokens
	}
	if settings.Temperature < 0 {
		settings.Temperature = 0
	}
	if settings.APIKeyEnv == "" {
		settings.APIKeyEnv = defaultAPIKeyEnv(settings.Provider)
	}
	if settings.Model == "" {
		settings.Model = defaultModel(settings.Provider)
	}
	if settings.EndpointVersion == "" {
		settings.EndpointVersion = defaultVersion(settings.Provider)
	}
	if settings.Endpoint == "" {
		settings.Endpoint = defaultEndpoint(settings.Provider, settings.Model, settings.EndpointVersion)
	}
	if settings.DefaultStatus == "" {
		settings.DefaultStatus = defaultGeneratedStatus
	}
	return settings
}

func (s Settings) redacted() Settings {
	s.APIKey = ""
	return s
}

func (s Settings) resolveAPIKey() (string, string) {
	if strings.TrimSpace(s.APIKey) != "" {
		return strings.TrimSpace(s.APIKey), "config"
	}
	envName := strings.TrimSpace(s.APIKeyEnv)
	if envName == "" {
		envName = defaultAPIKeyEnv(s.Provider)
	}
	if envName != "" {
		if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
			return value, "env:" + envName
		}
	}
	return "", "missing"
}

func requestCompletion(ctx context.Context, client *http.Client, settings Settings, prompt string) (string, TokenUsage, error) {
	switch strings.ToLower(strings.TrimSpace(settings.Provider)) {
	case "openai":
		return requestOpenAI(ctx, client, settings, prompt)
	case "anthropic", "claude":
		return requestAnthropic(ctx, client, settings, prompt)
	case "gemini", "google":
		return requestGemini(ctx, client, settings, prompt)
	default:
		return "", TokenUsage{}, fmt.Errorf("unsupported AI provider %q", settings.Provider)
	}
}

func requestOpenAI(ctx context.Context, client *http.Client, settings Settings, prompt string) (string, TokenUsage, error) {
	key, source := settings.resolveAPIKey()
	if key == "" {
		return "", TokenUsage{}, fmt.Errorf("OpenAI API key is not configured (%s)", source)
	}
	payload := map[string]any{
		"model":             settings.Model,
		"input":             prompt,
		"temperature":       settings.Temperature,
		"max_output_tokens": settings.MaxTokens,
	}
	var out struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Text string `json:"text"`
				Type string `json:"type"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := doJSON(ctx, client, http.MethodPost, settings.Endpoint, key, "bearer", payload, &out); err != nil {
		return "", TokenUsage{}, err
	}
	usage := normalizeUsage(out.Usage.InputTokens, out.Usage.OutputTokens, out.Usage.TotalTokens)
	if strings.TrimSpace(out.OutputText) != "" {
		return out.OutputText, usage, nil
	}
	var b strings.Builder
	for _, item := range out.Output {
		for _, content := range item.Content {
			if strings.TrimSpace(content.Text) != "" {
				b.WriteString(content.Text)
			}
		}
	}
	text, err := requireText(b.String(), "OpenAI")
	return text, usage, err
}

func requestAnthropic(ctx context.Context, client *http.Client, settings Settings, prompt string) (string, TokenUsage, error) {
	key, source := settings.resolveAPIKey()
	if key == "" {
		return "", TokenUsage{}, fmt.Errorf("Anthropic API key is not configured (%s)", source)
	}
	payload := map[string]any{
		"model":       settings.Model,
		"max_tokens":  settings.MaxTokens,
		"temperature": settings.Temperature,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := doJSON(ctx, client, http.MethodPost, settings.Endpoint, key, "anthropic", payload, &out); err != nil {
		return "", TokenUsage{}, err
	}
	usage := normalizeUsage(out.Usage.InputTokens, out.Usage.OutputTokens, 0)
	var b strings.Builder
	for _, content := range out.Content {
		if strings.TrimSpace(content.Text) != "" {
			b.WriteString(content.Text)
		}
	}
	text, err := requireText(b.String(), "Anthropic")
	return text, usage, err
}

func requestGemini(ctx context.Context, client *http.Client, settings Settings, prompt string) (string, TokenUsage, error) {
	key, source := settings.resolveAPIKey()
	if key == "" {
		return "", TokenUsage{}, fmt.Errorf("Gemini API key is not configured (%s)", source)
	}
	endpoint, err := endpointWithQueryKey(settings.Endpoint, key)
	if err != nil {
		return "", TokenUsage{}, err
	}
	payload := map[string]any{
		"contents": []map[string]any{
			{"role": "user", "parts": []map[string]any{{"text": prompt}}},
		},
		"generationConfig": map[string]any{
			"temperature":     settings.Temperature,
			"maxOutputTokens": settings.MaxTokens,
		},
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := doJSON(ctx, client, http.MethodPost, endpoint, "", "", payload, &out); err != nil {
		return "", TokenUsage{}, err
	}
	usage := normalizeUsage(out.UsageMetadata.PromptTokenCount, out.UsageMetadata.CandidatesTokenCount, out.UsageMetadata.TotalTokenCount)
	var b strings.Builder
	for _, candidate := range out.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				b.WriteString(part.Text)
			}
		}
	}
	text, err := requireText(b.String(), "Gemini")
	return text, usage, err
}

func doJSON(ctx context.Context, client *http.Client, method, endpoint, key, authStyle string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	switch authStyle {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+key)
	case "anthropic":
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, 4*1024*1024)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(limited)
		return fmt.Errorf("AI provider returned %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(limited).Decode(out); err != nil {
		return err
	}
	return nil
}

func normalizeUsage(prompt, completion, total int) TokenUsage {
	if total == 0 && (prompt != 0 || completion != 0) {
		total = prompt + completion
	}
	return TokenUsage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
	}
}

func buildPrompt(settings Settings, req GenerateRequest) string {
	var b strings.Builder
	b.WriteString("You are writing content for Foundry, a Markdown-driven CMS written in Go.\n")
	b.WriteString("Return exactly one complete Markdown post. Do not wrap the answer in code fences. Do not include commentary before or after the document.\n")
	b.WriteString("The document must start with YAML frontmatter between --- markers, followed by a polished Markdown body.\n")
	b.WriteString("Use frontmatter keys: title, slug, date, status, summary, tags, categories, author, lang, layout.\n")
	b.WriteString("Foundry will add AI provenance frontmatter automatically; do not invent provider usage metadata.\n")
	b.WriteString("Use layout: post. Use valid YAML arrays for tags and categories. Use status draft unless the request says otherwise.\n")
	b.WriteString("Write helpful, original prose with clear headings, concise paragraphs, and practical examples when useful.\n")
	if strings.TrimSpace(settings.SystemPrompt) != "" {
		b.WriteString("\nAdditional site instructions:\n")
		b.WriteString(strings.TrimSpace(settings.SystemPrompt))
		b.WriteString("\n")
	}
	b.WriteString("\nRequested post options:\n")
	appendPromptField(&b, "Title", req.Title)
	appendPromptField(&b, "Slug", req.Slug)
	appendPromptField(&b, "Status", firstNonEmpty(req.Status, settings.DefaultStatus))
	appendPromptField(&b, "Author", firstNonEmpty(req.Author, settings.DefaultAuthor))
	appendPromptField(&b, "Language", firstNonEmpty(req.Lang, settings.DefaultLang))
	appendPromptField(&b, "Summary", req.Summary)
	appendPromptList(&b, "Tags", req.Tags)
	appendPromptList(&b, "Categories", req.Categories)
	b.WriteString("\nUser prompt:\n")
	b.WriteString(strings.TrimSpace(req.Prompt))
	b.WriteString("\n")
	return b.String()
}

func completeMarkdownDocument(markdown string, settings Settings, req GenerateRequest) (string, string, string, string) {
	title := firstNonEmpty(req.Title, frontmatterValue(markdown, "title"), titleFromPrompt(req.Prompt))
	slug := slugify(firstNonEmpty(req.Slug, frontmatterValue(markdown, "slug"), title))
	status := lowerString(firstNonEmpty(req.Status, frontmatterValue(markdown, "status"), settings.DefaultStatus, defaultGeneratedStatus))
	author := firstNonEmpty(req.Author, frontmatterValue(markdown, "author"), settings.DefaultAuthor)
	lang := firstNonEmpty(req.Lang, frontmatterValue(markdown, "lang"), settings.DefaultLang)
	summary := firstNonEmpty(req.Summary, frontmatterValue(markdown, "summary"))

	if hasFrontmatter(markdown) {
		return markAIGenerated(markdown, settings), title, slug, status
	}

	var b strings.Builder
	b.WriteString("---\n")
	writeYAMLString(&b, "title", title)
	writeYAMLString(&b, "slug", slug)
	writeYAMLString(&b, "date", time.Now().UTC().Format(time.RFC3339))
	writeYAMLString(&b, "status", status)
	if summary != "" {
		writeYAMLString(&b, "summary", summary)
	}
	writeYAMLList(&b, "tags", req.Tags)
	writeYAMLList(&b, "categories", req.Categories)
	if author != "" {
		writeYAMLString(&b, "author", author)
	}
	if lang != "" {
		writeYAMLString(&b, "lang", lang)
	}
	writeYAMLString(&b, "layout", "post")
	writeYAMLBool(&b, "ai_generated", true)
	writeYAMLString(&b, "ai_provider", settings.Provider)
	writeYAMLString(&b, "ai_model", settings.Model)
	writeYAMLString(&b, "ai_generated_at", time.Now().UTC().Format(time.RFC3339))
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(markdown))
	b.WriteString("\n")
	return b.String(), title, slug, status
}

func markAIGenerated(markdown string, settings Settings) string {
	if !hasFrontmatter(markdown) || frontmatterValue(markdown, "ai_generated") != "" {
		return markdown
	}
	var b strings.Builder
	b.WriteString("---\n")
	writeYAMLBool(&b, "ai_generated", true)
	writeYAMLString(&b, "ai_provider", settings.Provider)
	writeYAMLString(&b, "ai_model", settings.Model)
	writeYAMLString(&b, "ai_generated_at", time.Now().UTC().Format(time.RFC3339))
	return strings.Replace(markdown, "---\n", b.String(), 1)
}

func writePost(cfg *config.Config, markdown string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required")
	}
	contentRoot := strings.TrimSpace(cfg.ContentDir)
	if contentRoot == "" {
		contentRoot = "content"
	}
	postsDir := strings.TrimSpace(cfg.Content.PostsDir)
	if postsDir == "" {
		postsDir = "posts"
	}
	postsRoot, err := safepath.ResolveRelativeUnderRoot(contentRoot, postsDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(postsRoot, 0o755); err != nil {
		return "", err
	}

	baseName := "ai-post-" + time.Now().UTC().Format("20060102-150405")
	for i := 0; i < 1000; i++ {
		name := baseName + ".md"
		if i > 0 {
			name = baseName + "-" + strconv.Itoa(i+1) + ".md"
		}
		target, err := safepath.ResolveRelativeUnderRoot(postsRoot, name)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.WriteFile(target, []byte(markdown), 0o644); err != nil {
			return "", err
		}
		return target, nil
	}
	return "", fmt.Errorf("could not allocate a unique post filename")
}

func countAIWrittenPosts(cfg *config.Config) int {
	root, err := postsRoot(cfg)
	if err != nil {
		return 0
	}
	count := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		target, err := safepath.ResolveRelativeUnderRoot(root, rel)
		if err != nil {
			return nil
		}
		b, err := os.ReadFile(target)
		if err != nil {
			return nil
		}
		if strings.EqualFold(frontmatterValue(string(b), "ai_generated"), "true") {
			count++
		}
		return nil
	})
	return count
}

func postsRoot(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required")
	}
	contentRoot := strings.TrimSpace(cfg.ContentDir)
	if contentRoot == "" {
		contentRoot = "content"
	}
	postsDir := strings.TrimSpace(cfg.Content.PostsDir)
	if postsDir == "" {
		postsDir = "posts"
	}
	return safepath.ResolveRelativeUnderRoot(contentRoot, postsDir)
}

func normalizeMarkdown(input string) string {
	out := strings.TrimSpace(input)
	if strings.HasPrefix(out, "```") {
		lines := strings.Split(out, "\n")
		if len(lines) >= 2 {
			first := strings.TrimSpace(lines[0])
			last := strings.TrimSpace(lines[len(lines)-1])
			if strings.HasPrefix(first, "```") && strings.HasPrefix(last, "```") {
				out = strings.Join(lines[1:len(lines)-1], "\n")
			}
		}
	}
	return strings.TrimSpace(out) + "\n"
}

func slugify(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	var b strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func hasFrontmatter(markdown string) bool {
	markdown = strings.TrimSpace(markdown)
	if !strings.HasPrefix(markdown, "---\n") {
		return false
	}
	return strings.Contains(markdown[4:], "\n---")
}

func frontmatterValue(markdown, key string) string {
	if !hasFrontmatter(markdown) {
		return ""
	}
	end := strings.Index(strings.TrimPrefix(markdown, "---\n"), "\n---")
	if end < 0 {
		return ""
	}
	block := strings.TrimPrefix(markdown, "---\n")[:end]
	for _, match := range frontmatterValuePattern.FindAllStringSubmatch(block, -1) {
		if strings.EqualFold(match[1], key) {
			return strings.TrimSpace(strings.Trim(match[2], `"'`))
		}
	}
	return ""
}

func endpointWithQueryKey(endpoint, key string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	query := u.Query()
	if query.Get("key") == "" {
		query.Set("key", key)
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func requireText(text, provider string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("%s response did not include generated text", provider)
	}
	return text, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	message := "request failed"
	if err != nil {
		message = err.Error()
	}
	writeJSON(w, status, map[string]string{"message": message})
}

func settingString(values map[string]any, key, fallback string) string {
	if values == nil {
		return fallback
	}
	switch v := values[key].(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	case fmt.Stringer:
		if strings.TrimSpace(v.String()) != "" {
			return strings.TrimSpace(v.String())
		}
	}
	return fallback
}

func settingInt(values map[string]any, key string, fallback int) int {
	switch v := values[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return fallback
}

func settingFloat(values map[string]any, key string, fallback float64) float64 {
	switch v := values[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func defaultAPIKeyEnv(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic", "claude":
		return "ANTHROPIC_API_KEY"
	case "gemini", "google":
		return "GEMINI_API_KEY"
	default:
		return "OPENAI_API_KEY"
	}
}

func isValidEnvVarName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for i, r := range value {
		if i == 0 && !(r == '_' || unicode.IsLetter(r)) {
			return false
		}
		if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

func defaultModel(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic", "claude":
		return defaultAnthropicModel
	case "gemini", "google":
		return defaultGeminiModel
	default:
		return defaultOpenAIModel
	}
}

func defaultVersion(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic", "claude":
		return defaultAnthropicVersion
	case "gemini", "google":
		return defaultGeminiVersion
	default:
		return defaultOpenAIVersion
	}
}

func normalizedEndpointVersion(provider, version string) string {
	version = strings.TrimSpace(version)
	if version != "" {
		return version
	}
	return defaultVersion(provider)
}

func resolveEndpoint(provider, model, endpointOverride, endpointVersionValue string) string {
	endpointOverride = strings.TrimSpace(endpointOverride)
	if endpointOverride == "" {
		return defaultEndpoint(provider, model, endpointVersionValue)
	}
	return strings.ReplaceAll(endpointOverride, "{version}", normalizedEndpointVersion(provider, endpointVersionValue))
}

func defaultEndpoint(provider, model, endpointVersionValue string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic", "claude":
		return strings.ReplaceAll(defaultAnthropicURL, "{version}", normalizedEndpointVersion(provider, endpointVersionValue))
	case "gemini", "google":
		endpoint := strings.ReplaceAll(defaultGeminiEndpoint, "{version}", normalizedEndpointVersion(provider, endpointVersionValue))
		return fmt.Sprintf(endpoint, url.PathEscape(firstNonEmpty(model, defaultGeminiModel)))
	default:
		return strings.ReplaceAll(defaultOpenAIEndpoint, "{version}", normalizedEndpointVersion(provider, endpointVersionValue))
	}
}

func lowerString(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func appendPromptField(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	b.WriteString("- ")
	b.WriteString(label)
	b.WriteString(": ")
	b.WriteString(strings.TrimSpace(value))
	b.WriteString("\n")
}

func appendPromptList(b *strings.Builder, label string, values []string) {
	cleaned := cleanStringList(values)
	if len(cleaned) == 0 {
		return
	}
	appendPromptField(b, label, strings.Join(cleaned, ", "))
}

func writeYAMLString(b *strings.Builder, key, value string) {
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(strconv.Quote(strings.TrimSpace(value)))
	b.WriteString("\n")
}

func writeYAMLBool(b *strings.Builder, key string, value bool) {
	b.WriteString(key)
	if value {
		b.WriteString(": true\n")
		return
	}
	b.WriteString(": false\n")
}

func writeYAMLList(b *strings.Builder, key string, values []string) {
	cleaned := cleanStringList(values)
	if len(cleaned) == 0 {
		b.WriteString(key)
		b.WriteString(": []\n")
		return
	}
	b.WriteString(key)
	b.WriteString(":\n")
	for _, value := range cleaned {
		b.WriteString("  - ")
		b.WriteString(strconv.Quote(value))
		b.WriteString("\n")
	}
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func titleFromPrompt(prompt string) string {
	fields := strings.Fields(strings.TrimSpace(prompt))
	if len(fields) == 0 {
		return "AI Generated Post"
	}
	if len(fields) > 8 {
		fields = fields[:8]
	}
	title := strings.Join(fields, " ")
	title = strings.Trim(title, ".,:;!?\"'")
	if title == "" {
		return "AI Generated Post"
	}
	return title
}

func init() {
	plugins.Register(pluginName, func() plugins.Plugin { return New() })
}
