document.addEventListener('foundry:admin-extension-page', async (event) => {
  const detail = event.detail || {};
  const mount = document.getElementById(detail.mountId || 'admin-extension-mount');
  if (!mount || !window.FoundryAdmin) {
    return;
  }

  const { client } = window.FoundryAdmin;
  const page = detail.page || {};
  const capabilities = detail.capabilities || [];

  mount.innerHTML = `
    <div class="stack">
      <h3>${page.title || page.key || 'Plugin Page'}</h3>
      <div class="muted">Plugin: ${page.plugin || 'unknown'}</div>
      <div class="muted">Capabilities: ${capabilities.join(', ') || 'none'}</div>
      <button type="button" id="plugin-extension-refresh">Refresh status</button>
      <pre id="plugin-extension-output">Loading…</pre>
    </div>
  `;

  const output = mount.querySelector('#plugin-extension-output');
  const refresh = async () => {
    try {
      const status = await client.status.get();
      output.textContent = JSON.stringify(status, null, 2);
    } catch (error) {
      output.textContent = error?.message || String(error);
    }
  };

  mount.querySelector('#plugin-extension-refresh')?.addEventListener('click', () => {
    void refresh();
  });

  await refresh();
});
