document.addEventListener('foundry:admin-extension-widget', async (event) => {
  const detail = event.detail || {};
  const mount = document.getElementById(detail.mountId || '');
  if (!mount || !window.FoundryAdmin) {
    return;
  }

  const { client } = window.FoundryAdmin;
  const widget = detail.widget || {};

  mount.innerHTML = `
    <div class="stack">
      <div class="muted">Widget: ${widget.title || widget.key || 'unnamed'}</div>
      <button type="button" id="${detail.mountId}-refresh">Refresh</button>
      <pre id="${detail.mountId}-output">Loading…</pre>
    </div>
  `;

  const output = mount.querySelector(`#${CSS.escape(detail.mountId)}-output`);
  const refreshButton = mount.querySelector(`#${CSS.escape(detail.mountId)}-refresh`);

  const refresh = async () => {
    try {
      const status = await client.status.get();
      output.textContent = JSON.stringify(
        { title: status.title, content: status.content },
        null,
        2
      );
    } catch (error) {
      output.textContent = error?.message || String(error);
    }
  };

  refreshButton?.addEventListener('click', () => {
    void refresh();
  });

  await refresh();
});
