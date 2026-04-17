# AI Writer

AI Writer adds an authenticated admin page that generates Foundry Markdown posts with an AI provider. It supports OpenAI, Anthropic Claude, and Google Gemini through server-side HTTP calls so API keys are never exposed to browser JavaScript.

Generated output is written as a new Markdown file under the configured Foundry posts directory. Existing files are not overwritten. The frontmatter slug controls the post URL, while Foundry generates the filename so user input never controls the filesystem path.

## What It Does

- Adds an **AI Writer** page to the Admin UI under the content navigation group.
- Lets an admin enter a post prompt plus optional title, slug, status, language, author, summary, tags, and categories.
- Wraps the user prompt in Foundry-specific instructions so the provider returns one complete Markdown post with YAML frontmatter.
- Calls OpenAI, Anthropic Claude, or Gemini from the server.
- Writes the generated Markdown as a draft-style post in `content/<posts_dir>/` using a server-generated filename.
- Marks generated posts with AI provenance frontmatter so they can be counted and audited later.
- Shows provider-reported token usage for the last request and for the current Foundry process.

## Enable The Plugin

Add `aiwriter` to the enabled plugin list in your Foundry site config:

```yaml
plugins:
  enabled:
    - aiwriter
```

If you are running from source, sync plugin imports and rebuild after enabling:

```sh
go run ./cmd/plugin-sync
go build ./cmd/foundry
```

## Configuration

Configure the plugin under `params.ai_writer`. API keys should normally come from environment variables, not from the config file.

```yaml
params:
  ai_writer:
    provider: openai
    api_key_env: OPENAI_API_KEY
    model: gpt-4o-mini
    endpoint_version: v1
    temperature: 0.7
    max_tokens: 2400
    timeout_seconds: 60
    default_status: draft
    default_author: Foundry Team
    default_lang: en
    system_prompt: >
      Match our site voice: direct, practical, and concise.
```

### Provider Examples

OpenAI:

```yaml
params:
  ai_writer:
    provider: openai
    api_key_env: OPENAI_API_KEY
    model: gpt-4o-mini
```

Anthropic Claude:

```yaml
params:
  ai_writer:
    provider: anthropic
    api_key_env: ANTHROPIC_API_KEY
    model: claude-3-5-haiku-latest
```

Google Gemini:

```yaml
params:
  ai_writer:
    provider: gemini
    api_key_env: GEMINI_API_KEY
    model: gemini-1.5-flash
```

## Options

- `provider`: `openai`, `anthropic`, `claude`, `gemini`, or `google`.
- `api_key_env`: Environment variable that contains the provider API key.
- `api_key`: Inline API key. Supported, but not recommended for production.
- `model`: Provider model name.
- `endpoint`: Optional provider endpoint override, useful for tests or gateways.
- `endpoint_version`: API endpoint version for the provider default endpoint. Defaults are `v1` for OpenAI and Anthropic, and `v1beta` for Gemini. Custom endpoint overrides can include `{version}` to reuse this value.
- `temperature`: Provider sampling temperature.
- `max_tokens`: Maximum output token budget.
- `timeout_seconds`: HTTP timeout for provider requests.
- `default_status`: Default frontmatter status when the admin form does not override it.
- `default_author`: Default frontmatter author.
- `default_lang`: Default frontmatter language.
- `system_prompt`: Additional site-specific writing instructions appended to the built-in Foundry prompt.

## Admin Usage

Open the Admin UI and select **AI Writer**. Enter the prompt and any optional metadata. The generated post is written immediately when you press **Generate and write post**, and the page shows the saved path plus the generated Markdown preview.

The route requires an authenticated admin session with `documents.create`.

## AI Provenance

Generated posts include frontmatter like this:

```yaml
ai_generated: true
ai_provider: openai
ai_model: gpt-4o-mini
ai_generated_at: "2026-04-16T19:00:00Z"
```

The Admin UI counts posts with `ai_generated: true` and displays provider-reported token usage when the provider returns it. Runtime token totals are kept in memory for the current Foundry process; the frontmatter marker is durable.

## Security Notes

- API keys stay on the server. The Admin UI only receives a redacted configuration status.
- The plugin only writes inside Foundry's configured content posts directory with server-generated filenames.
- Existing Markdown files are not overwritten.
- Provider calls are outbound HTTPS requests to the configured AI provider.
- The plugin requires admin approval because it combines outbound network access with content writes.
