package aiwriter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func TestSettingsFromConfig(t *testing.T) {
	cfg := &config.Config{
		DefaultLang: "en",
		Params: map[string]any{
			"ai_writer": map[string]any{
				"provider":         "gemini",
				"api_key_env":      "CUSTOM_GEMINI_KEY",
				"model":            "gemini-2.0-flash",
				"endpoint_version": "v1",
				"temperature":      0.4,
				"max_tokens":       1200,
				"timeout_seconds":  12,
				"default_status":   "review",
			},
		},
	}

	settings := settingsFromConfig(cfg)
	if settings.Provider != "gemini" {
		t.Fatalf("expected gemini provider, got %q", settings.Provider)
	}
	if settings.APIKeyEnv != "CUSTOM_GEMINI_KEY" {
		t.Fatalf("expected custom key env, got %q", settings.APIKeyEnv)
	}
	if settings.Model != "gemini-2.0-flash" {
		t.Fatalf("expected configured model, got %q", settings.Model)
	}
	if settings.EndpointVersion != "v1" {
		t.Fatalf("expected configured endpoint version, got %q", settings.EndpointVersion)
	}
	if settings.Endpoint != "https://generativelanguage.googleapis.com/v1/models/gemini-2.0-flash:generateContent" {
		t.Fatalf("unexpected endpoint: %q", settings.Endpoint)
	}
	if settings.DefaultStatus != "review" {
		t.Fatalf("expected review default status, got %q", settings.DefaultStatus)
	}
	if settings.DefaultLang != "en" {
		t.Fatalf("expected default language from config, got %q", settings.DefaultLang)
	}
}

func TestDefaultEndpointVersions(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
		version  string
		want     string
	}{
		{
			name:     "openai default",
			provider: "openai",
			model:    "gpt-test",
			want:     "https://api.openai.com/v1/responses",
		},
		{
			name:     "openai override",
			provider: "openai",
			model:    "gpt-test",
			version:  "v2",
			want:     "https://api.openai.com/v2/responses",
		},
		{
			name:     "anthropic default",
			provider: "anthropic",
			model:    "claude-test",
			want:     "https://api.anthropic.com/v1/messages",
		},
		{
			name:     "gemini default",
			provider: "gemini",
			model:    "gemini test/model",
			want:     "https://generativelanguage.googleapis.com/v1beta/models/gemini%20test%2Fmodel:generateContent",
		},
		{
			name:     "gemini override",
			provider: "gemini",
			model:    "gemini-test",
			version:  "v1",
			want:     "https://generativelanguage.googleapis.com/v1/models/gemini-test:generateContent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultEndpoint(tt.provider, tt.model, tt.version); got != tt.want {
				t.Fatalf("defaultEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveEndpointAppliesVersionPlaceholderToOverrides(t *testing.T) {
	got := resolveEndpoint("openai", "ignored", "https://gateway.example/{version}/responses", "v9")
	if got != "https://gateway.example/v9/responses" {
		t.Fatalf("unexpected endpoint override: %q", got)
	}
	got = resolveEndpoint("openai", "ignored", "https://gateway.example/static", "v9")
	if got != "https://gateway.example/static" {
		t.Fatalf("unexpected static endpoint override: %q", got)
	}
}

func TestSettingsFromConfigTreatsInvalidAPIKeyEnvAsInlineKey(t *testing.T) {
	cfg := &config.Config{
		Params: map[string]any{
			"ai_writer": map[string]any{
				"provider":    "openai",
				"api_key_env": "sk-test-temporary-key",
			},
		},
	}

	settings := settingsFromConfig(cfg)
	if settings.APIKey != "sk-test-temporary-key" {
		t.Fatal("expected invalid api_key_env value to be treated as inline API key")
	}
	if settings.APIKeyEnv != "OPENAI_API_KEY" {
		t.Fatalf("expected api_key_env to reset to default env var name, got %q", settings.APIKeyEnv)
	}
	key, source := settings.resolveAPIKey()
	if key == "" || source != "config" {
		t.Fatalf("expected config key source, got key-present=%v source=%q", key != "", source)
	}
}

func TestIsValidEnvVarName(t *testing.T) {
	for _, value := range []string{"OPENAI_API_KEY", "_PRIVATE_KEY", "A1"} {
		if !isValidEnvVarName(value) {
			t.Fatalf("expected %q to be valid", value)
		}
	}
	for _, value := range []string{"", "1OPENAI", "sk-test-key", "OPENAI API KEY"} {
		if isValidEnvVarName(value) {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}

func TestBuildPromptWrapsUserPromptWithFoundryInstructions(t *testing.T) {
	prompt := buildPrompt(Settings{DefaultStatus: "draft", DefaultLang: "en"}, GenerateRequest{
		Prompt: "Write about launch checklists.",
		Title:  "Launch checklist",
		Tags:   []string{"cms", "launch"},
	})

	for _, want := range []string{
		"Foundry, a Markdown-driven CMS written in Go",
		"Return exactly one complete Markdown post",
		"Title: Launch checklist",
		"Tags: cms, launch",
		"Write about launch checklists.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestCompleteMarkdownDocumentAddsFrontmatter(t *testing.T) {
	markdown, title, slug, status := completeMarkdownDocument("# Body\n\nHello.", Settings{
		DefaultStatus: "draft",
		DefaultAuthor: "Foundry Team",
		DefaultLang:   "en",
	}, GenerateRequest{
		Title:      "A Useful Post",
		Prompt:     "A useful post",
		Tags:       []string{"one", "two"},
		Categories: []string{"Guides"},
	})

	if title != "A Useful Post" || slug != "a-useful-post" || status != "draft" {
		t.Fatalf("unexpected metadata title=%q slug=%q status=%q", title, slug, status)
	}
	for _, want := range []string{
		"---\n",
		`title: "A Useful Post"`,
		`slug: "a-useful-post"`,
		`author: "Foundry Team"`,
		`lang: "en"`,
		`layout: "post"`,
		"ai_generated: true",
		"# Body",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestRequestOpenAI(t *testing.T) {
	var sawPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		sawPrompt, _ = body["input"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"---\ntitle: Generated\n---\n\n# Generated\n","usage":{"input_tokens":11,"output_tokens":17,"total_tokens":28}}`))
	}))
	defer server.Close()

	got, usage, err := requestCompletion(context.Background(), server.Client(), Settings{
		Provider:    "openai",
		APIKey:      "test-key",
		Model:       "test-model",
		Endpoint:    server.URL,
		Temperature: 0.2,
		MaxTokens:   200,
	}, "wrapped prompt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "# Generated") {
		t.Fatalf("unexpected generated markdown: %s", got)
	}
	if usage.TotalTokens != 28 || usage.PromptTokens != 11 || usage.CompletionTokens != 17 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	if sawPrompt != "wrapped prompt" {
		t.Fatalf("unexpected prompt %q", sawPrompt)
	}
}

func TestRequestAnthropic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Fatalf("unexpected api key header %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatal("expected anthropic-version header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"# Claude Post"}],"usage":{"input_tokens":13,"output_tokens":21}}`))
	}))
	defer server.Close()

	got, usage, err := requestCompletion(context.Background(), server.Client(), Settings{
		Provider:    "anthropic",
		APIKey:      "test-key",
		Model:       "claude-test",
		Endpoint:    server.URL,
		Temperature: 0.2,
		MaxTokens:   200,
	}, "wrapped prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "# Claude Post" {
		t.Fatalf("unexpected generated markdown: %s", got)
	}
	if usage.TotalTokens != 34 || usage.PromptTokens != 13 || usage.CompletionTokens != 21 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
}

func TestRequestGemini(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("key"); got != "test-key" {
			t.Fatalf("unexpected query key %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"# Gemini Post"}]}}],"usageMetadata":{"promptTokenCount":9,"candidatesTokenCount":14,"totalTokenCount":23}}`))
	}))
	defer server.Close()

	got, usage, err := requestCompletion(context.Background(), server.Client(), Settings{
		Provider:    "gemini",
		APIKey:      "test-key",
		Model:       "gemini-test",
		Endpoint:    server.URL,
		Temperature: 0.2,
		MaxTokens:   200,
	}, "wrapped prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "# Gemini Post" {
		t.Fatalf("unexpected generated markdown: %s", got)
	}
	if usage.TotalTokens != 23 || usage.PromptTokens != 9 || usage.CompletionTokens != 14 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
}

func TestGenerateAndWriteCreatesUniquePost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"# Body\n\nGenerated content.","usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`))
	}))
	defer server.Close()

	tmp := t.TempDir()
	cfg := &config.Config{
		ContentDir: tmp,
		Content:    config.ContentConfig{PostsDir: "posts"},
		Params: map[string]any{
			"ai_writer": map[string]any{
				"provider": "openai",
				"api_key":  "test-key",
				"model":    "test-model",
				"endpoint": server.URL,
			},
		},
	}
	plugin := New()
	if err := plugin.OnConfigLoaded(cfg); err != nil {
		t.Fatal(err)
	}

	first, err := plugin.GenerateAndWrite(context.Background(), GenerateRequest{
		Prompt: "Generate one post.",
		Title:  "Generated Post",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := plugin.GenerateAndWrite(context.Background(), GenerateRequest{
		Prompt: "Generate another post.",
		Title:  "Generated Post",
	})
	if err != nil {
		t.Fatal(err)
	}

	if first.Path == second.Path {
		t.Fatalf("expected unique paths, both were %q", first.Path)
	}
	for _, path := range []string{first.Path, second.Path} {
		b, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `title: "Generated Post"`) {
			t.Fatalf("generated file missing frontmatter: %s", string(b))
		}
		if !strings.Contains(string(b), "ai_generated: true") {
			t.Fatalf("generated file missing AI marker: %s", string(b))
		}
	}
	if count := countAIWrittenPosts(cfg); count != 2 {
		t.Fatalf("expected 2 AI-written posts, got %d", count)
	}
	usage := plugin.usageSnapshot()
	if usage.GenerationsThisRun != 2 || usage.TotalTokensThisRun != 14 {
		t.Fatalf("unexpected process usage: %#v", usage)
	}
}
