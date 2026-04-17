const splitList = (value) =>
  String(value || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);

const renderSettings = (settings) => {
  if (!settings) return 'Settings unavailable';
  const configured = settings.configured ? 'configured' : 'missing API key';
  return `${settings.provider || 'openai'} / ${settings.model || 'default'} (${configured}, ${settings.key_source || 'unknown'})`;
};

const formatNumber = (value) => new Intl.NumberFormat().format(Number(value || 0));

const formatUsageLine = (usage = {}) =>
  `${formatNumber(usage.prompt_tokens || 0)} prompt / ${formatNumber(usage.completion_tokens || 0)} completion / ${formatNumber(usage.total_tokens || 0)} total`;

export const mountAdminExtensionPage = async ({
  mount,
  page = {},
  client,
} = {}) => {
  if (page.key !== 'ai-writer') return;

  client = client || window.FoundryAdmin?.client;
  if (!mount || !client?.raw) return;

  mount.innerHTML = `
    <section class="aiwriter-shell">
      <div class="aiwriter-hero">
        <div>
          <p class="aiwriter-eyebrow">AI Writer</p>
          <h2>Generate a Foundry Markdown post</h2>
          <p class="aiwriter-muted">Prompt an approved provider from the admin UI. API keys stay server-side and generated posts are written as draft Markdown files under the configured posts directory.</p>
        </div>
        <div class="aiwriter-provider-card">
          <span class="aiwriter-label">Provider</span>
          <strong id="aiwriter-provider-status">Loading settings...</strong>
        </div>
      </div>

      <div class="aiwriter-stats">
        <article>
          <span class="aiwriter-label">AI-written posts</span>
          <strong id="aiwriter-ai-post-count">-</strong>
          <p class="aiwriter-muted">Posts marked with <code>ai_generated: true</code>.</p>
        </article>
        <article>
          <span class="aiwriter-label">This process</span>
          <strong id="aiwriter-run-generations">-</strong>
          <p class="aiwriter-muted">Generated since the current Foundry process started.</p>
        </article>
        <article>
          <span class="aiwriter-label">Token usage</span>
          <strong id="aiwriter-run-tokens">-</strong>
          <p class="aiwriter-muted">Provider-reported tokens for this process.</p>
        </article>
        <article>
          <span class="aiwriter-label">Last request</span>
          <strong id="aiwriter-last-tokens">-</strong>
          <p class="aiwriter-muted">Prompt / completion / total tokens.</p>
        </article>
      </div>

      <form id="aiwriter-form" class="aiwriter-grid">
        <label class="aiwriter-full">
          <span>What should the post be about?</span>
          <textarea name="prompt" rows="8" required placeholder="Example: Write a practical guide for small teams migrating a documentation site into Foundry. Include tradeoffs, workflow tips, and a launch checklist."></textarea>
        </label>

        <label>
          <span>Title override</span>
          <input name="title" type="text" placeholder="Optional">
        </label>

        <label>
          <span>Slug override</span>
          <input name="slug" type="text" placeholder="optional-post-slug">
        </label>

        <label>
          <span>Status</span>
          <select name="status">
            <option value="draft">Draft</option>
            <option value="review">Review</option>
            <option value="published">Published</option>
          </select>
        </label>

        <label>
          <span>Language</span>
          <input name="lang" type="text" placeholder="en">
        </label>

        <label>
          <span>Author</span>
          <input name="author" type="text" placeholder="Optional">
        </label>

        <label>
          <span>Summary</span>
          <input name="summary" type="text" placeholder="Optional frontmatter summary">
        </label>

        <label>
          <span>Tags</span>
          <input name="tags" type="text" placeholder="comma, separated, tags">
        </label>

        <label>
          <span>Categories</span>
          <input name="categories" type="text" placeholder="comma, separated, categories">
        </label>

        <div class="aiwriter-actions aiwriter-full">
          <button type="submit" class="aiwriter-primary">Generate and write post</button>
          <button type="button" id="aiwriter-clear">Clear</button>
        </div>
      </form>

      <div id="aiwriter-message" class="aiwriter-message" hidden></div>

      <section id="aiwriter-result" class="aiwriter-result" hidden>
        <div class="aiwriter-result-header">
          <div>
            <p class="aiwriter-eyebrow">Generated Post</p>
            <h3 id="aiwriter-result-title"></h3>
            <p id="aiwriter-result-meta" class="aiwriter-muted"></p>
          </div>
        </div>
        <pre id="aiwriter-markdown"></pre>
      </section>
    </section>
  `;

  const status = mount.querySelector('#aiwriter-provider-status');
  const aiPostCount = mount.querySelector('#aiwriter-ai-post-count');
  const runGenerations = mount.querySelector('#aiwriter-run-generations');
  const runTokens = mount.querySelector('#aiwriter-run-tokens');
  const lastTokens = mount.querySelector('#aiwriter-last-tokens');
  const form = mount.querySelector('#aiwriter-form');
  const message = mount.querySelector('#aiwriter-message');
  const result = mount.querySelector('#aiwriter-result');
  const resultTitle = mount.querySelector('#aiwriter-result-title');
  const resultMeta = mount.querySelector('#aiwriter-result-meta');
  const markdown = mount.querySelector('#aiwriter-markdown');

  const showMessage = (text, type = 'info') => {
    message.textContent = text;
    message.dataset.type = type;
    message.hidden = false;
  };

  const clearMessage = () => {
    message.textContent = '';
    message.hidden = true;
  };

  const applySettings = (settings) => {
    status.textContent = renderSettings(settings);
    const usage = settings.usage || {};
    aiPostCount.textContent = formatNumber(usage.ai_written_posts || 0);
    runGenerations.textContent = formatNumber(usage.generations_this_run || 0);
    runTokens.textContent = formatUsageLine({
      prompt_tokens: usage.prompt_tokens_this_run,
      completion_tokens: usage.completion_tokens_this_run,
      total_tokens: usage.total_tokens_this_run,
    });
    lastTokens.textContent = formatUsageLine(usage.last_request || {});
  };

  const refreshSettings = async () => {
    const settings = await client.raw.get('/plugin-api/aiwriter/settings');
    applySettings(settings);
    return settings;
  };

  try {
    const settings = await refreshSettings();
    if (!settings.configured) {
      showMessage('Configure an API key environment variable or params.ai_writer.api_key before generating posts.', 'warning');
    }
  } catch (error) {
    status.textContent = 'Settings unavailable';
    showMessage(error?.message || String(error), 'error');
  }

  mount.querySelector('#aiwriter-clear')?.addEventListener('click', () => {
    form.reset();
    result.hidden = true;
    clearMessage();
  });

  form.addEventListener('submit', async (submitEvent) => {
    submitEvent.preventDefault();
    clearMessage();
    result.hidden = true;

    const submit = form.querySelector('button[type="submit"]');
    const formData = new FormData(form);
    const payload = {
      prompt: formData.get('prompt'),
      title: formData.get('title'),
      slug: formData.get('slug'),
      status: formData.get('status'),
      lang: formData.get('lang'),
      author: formData.get('author'),
      summary: formData.get('summary'),
      tags: splitList(formData.get('tags')),
      categories: splitList(formData.get('categories')),
    };

    submit.disabled = true;
    submit.textContent = 'Generating...';
    try {
      const generated = await client.raw.post('/plugin-api/aiwriter/generate', payload);
      resultTitle.textContent = generated.title || 'Generated post';
      resultMeta.textContent = `${generated.path || 'unknown path'} · ${generated.provider || 'provider'} / ${generated.model || 'model'} · ${generated.status || 'draft'} · ${formatUsageLine(generated.usage || {})}`;
      markdown.textContent = generated.markdown || '';
      result.hidden = false;
      await refreshSettings();
      showMessage(`Wrote ${generated.path || 'the generated post'}`, 'success');
    } catch (error) {
      showMessage(error?.message || String(error), 'error');
    } finally {
      submit.disabled = false;
      submit.textContent = 'Generate and write post';
    }
  });
};

document.addEventListener('foundry:admin-extension-page', (event) => {
  const detail = event.detail || {};
  void mountAdminExtensionPage({
    mount: document.getElementById(detail.mountId || 'admin-extension-mount'),
    page: detail.page || {},
    client: window.FoundryAdmin?.client,
  });
});
