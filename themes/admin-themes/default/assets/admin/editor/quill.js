const loadStylesheet = (href) => {
  if (
    [...document.querySelectorAll('link[data-quill-style]')].some(
      (link) => link.dataset.quillStyle === href
    )
  ) {
    return Promise.resolve();
  }
  return new Promise((resolve, reject) => {
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = href;
    link.dataset.quillStyle = href;
    link.onload = () => resolve();
    link.onerror = () => reject(new Error(`failed to load stylesheet: ${href}`));
    document.head.appendChild(link);
  });
};

const loadScript = (src) => {
  if (window.Quill) {
    return Promise.resolve(window.Quill);
  }
  return new Promise((resolve, reject) => {
    const existing = [...document.querySelectorAll('script[data-quill-script]')].find(
      (script) => script.dataset.quillScript === src
    );
    if (existing) {
      existing.addEventListener('load', () => resolve(window.Quill), { once: true });
      existing.addEventListener('error', () => reject(new Error(`failed to load script: ${src}`)), {
        once: true,
      });
      return;
    }

    const script = document.createElement('script');
    script.src = src;
    script.async = true;
    script.dataset.quillScript = src;
    script.onload = () => resolve(window.Quill);
    script.onerror = () => reject(new Error(`failed to load script: ${src}`));
    document.head.appendChild(script);
  });
};

const escapeHTML = (value) =>
  String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');

const escapeMarkdownText = (value) =>
  String(value ?? '')
    .replaceAll('\\', '\\\\')
    .replaceAll('`', '\\`')
    .replaceAll('*', '\\*')
    .replaceAll('_', '\\_')
    .replaceAll('{', '\\{')
    .replaceAll('}', '\\}')
    .replaceAll('[', '\\[')
    .replaceAll(']', '\\]')
    .replaceAll('(', '\\(')
    .replaceAll(')', '\\)')
    .replaceAll('#', '\\#')
    .replaceAll('+', '\\+')
    .replaceAll('-', '\\-')
    .replaceAll('!', '\\!')
    .replaceAll('>', '\\>')
    .replaceAll('|', '\\|');

const normalizeMarkdownSpacing = (value) =>
  String(value || '')
    .replace(/\n{3,}/g, '\n\n')
    .replace(/[ \t]+\n/g, '\n')
    .trim();

const inlineMarkdownToHTML = (value, mediaItems = []) => {
  let text = escapeHTML(String(value ?? ''));
  const resolveMediaURL = (src) => {
    const normalized = String(src ?? '').trim();
    const item = mediaItems.find(
      (entry) =>
        entry.reference === normalized ||
        entry.public_url === normalized ||
        entry.public_url === normalized.replace(/^https?:\/\/[^/]+/, '')
    );
    if (item?.public_url) return item.public_url;
    return normalized;
  };
  text = text.replace(
    /!\[([^\]]*)\]\(([^)\s]+)(?:\s+"([^"]*)")?\)/g,
    (_match, alt, src) => `<img alt="${escapeHTML(alt)}" src="${escapeHTML(resolveMediaURL(src))}">`
  );
  text = text.replace(
    /\[([^\]]+)\]\(([^)\s]+)(?:\s+"([^"]*)")?\)/g,
    (_match, label, href) => `<a href="${escapeHTML(resolveMediaURL(href))}">${label}</a>`
  );
  text = text.replace(/`([^`]+)`/g, (_match, code) => `<code>${escapeHTML(code)}</code>`);
  text = text.replace(/\*\*([^*]+)\*\*/g, (_match, bold) => `<strong>${bold}</strong>`);
  text = text.replace(/~~([^~]+)~~/g, (_match, strike) => `<s>${strike}</s>`);
  text = text.replace(/\*([^*]+)\*/g, (_match, italic) => `<em>${italic}</em>`);
  text = text.replace(/_([^_]+)_/g, (_match, italic) => `<em>${italic}</em>`);
  return text.replace(/\n/g, '<br>');
};

const blockMarkdownToHTML = (markdown, mediaItems = []) => {
  const body = String(markdown ?? '')
    .replaceAll('\r\n', '\n')
    .trim();
  if (!body) return '<p><br></p>';

  const lines = body.split('\n');
  const parts = [];

  const pushParagraph = (paragraphLines) => {
    const text = paragraphLines.join(' ').trim();
    if (!text) return;
    parts.push(`<p>${inlineMarkdownToHTML(text, mediaItems)}</p>`);
  };

  let index = 0;
  while (index < lines.length) {
    const line = lines[index];
    if (!line.trim()) {
      index += 1;
      continue;
    }

    const fenceMatch = line.match(/^(```|~~~)(.*)$/);
    if (fenceMatch) {
      const fence = fenceMatch[1];
      const codeLines = [];
      index += 1;
      while (index < lines.length && !lines[index].startsWith(fence)) {
        codeLines.push(lines[index]);
        index += 1;
      }
      if (index < lines.length) index += 1;
      parts.push(`<pre><code>${escapeHTML(codeLines.join('\n'))}</code></pre>`);
      continue;
    }

    const headingMatch = line.match(/^(#{1,6})\s+(.*)$/);
    if (headingMatch) {
      const level = headingMatch[1].length;
      parts.push(`<h${level}>${inlineMarkdownToHTML(headingMatch[2], mediaItems)}</h${level}>`);
      index += 1;
      continue;
    }

    if (/^>\s?/.test(line)) {
      const quoteLines = [];
      while (index < lines.length && /^>\s?/.test(lines[index])) {
        quoteLines.push(lines[index].replace(/^>\s?/, ''));
        index += 1;
      }
      parts.push(
        `<blockquote>${blockMarkdownToHTML(quoteLines.join('\n'), mediaItems)}</blockquote>`
      );
      continue;
    }

    const listMatch = line.match(/^(\s*)([-*+]|\d+\.)\s+(.*)$/);
    if (listMatch) {
      const ordered = /\d+\./.test(listMatch[2]);
      const items = [];
      while (index < lines.length) {
        const current = lines[index];
        const currentMatch = current.match(/^(\s*)([-*+]|\d+\.)\s+(.*)$/);
        if (!currentMatch || Boolean(/\d+\./.test(currentMatch[2]) !== ordered)) break;
        const itemLines = [currentMatch[3]];
        index += 1;
        while (index < lines.length && lines[index].startsWith('  ')) {
          itemLines.push(lines[index].trim());
          index += 1;
        }
        items.push(`<li>${inlineMarkdownToHTML(itemLines.join(' ').trim(), mediaItems)}</li>`);
      }
      parts.push(`<${ordered ? 'ol' : 'ul'}>${items.join('')}</${ordered ? 'ol' : 'ul'}>`);
      continue;
    }

    const paragraphLines = [line];
    index += 1;
    while (index < lines.length) {
      const next = lines[index];
      if (!next.trim()) break;
      if (/^#{1,6}\s+/.test(next) || /^>\s?/.test(next) || /^(```|~~~)/.test(next)) break;
      if (/^(\s*)([-*+]|\d+\.)\s+/.test(next)) break;
      paragraphLines.push(next);
      index += 1;
    }
    pushParagraph(paragraphLines);
  }

  return parts.join('\n');
};

const resolveMediaReference = (src, state, uploadedMediaByURL) => {
  const normalized = String(src ?? '').trim();
  if (!normalized) return '';
  if (uploadedMediaByURL?.has(normalized)) {
    return uploadedMediaByURL.get(normalized);
  }
  const mediaItem = (state.media || []).find(
    (entry) =>
      entry.public_url === normalized ||
      entry.reference === normalized ||
      entry.public_url === normalized.replace(/^https?:\/\/[^/]+/, '')
  );
  if (mediaItem?.reference) {
    return mediaItem.reference;
  }
  return normalized;
};

const serializeInlineNode = (node, state, uploadedMediaByURL) => {
  if (!node) return '';
  if (node.nodeType === Node.TEXT_NODE) {
    return escapeMarkdownText(node.textContent || '');
  }
  if (node.nodeType !== Node.ELEMENT_NODE) {
    return '';
  }

  const tag = node.tagName.toUpperCase();
  const inlineChildren = () =>
    Array.from(node.childNodes)
      .map((child) => serializeInlineNode(child, state, uploadedMediaByURL))
      .join('');

  switch (tag) {
    case 'BR':
      return '  \n';
    case 'STRONG':
    case 'B':
      return `**${inlineChildren()}**`;
    case 'EM':
    case 'I':
      return `*${inlineChildren()}*`;
    case 'S':
    case 'DEL':
      return `~~${inlineChildren()}~~`;
    case 'CODE':
      return `\`${(node.textContent || '').replaceAll('`', '\\`')}\``;
    case 'A': {
      const href = resolveMediaReference(
        node.getAttribute('href') || '',
        state,
        uploadedMediaByURL
      );
      const label = inlineChildren();
      return `[${label}](${href})`;
    }
    case 'IMG': {
      const src = resolveMediaReference(node.getAttribute('src') || '', state, uploadedMediaByURL);
      const alt = node.getAttribute('alt') || node.getAttribute('title') || '';
      return `![${escapeMarkdownText(alt)}](${src})`;
    }
    case 'SPAN':
    case 'U':
      return inlineChildren();
    default:
      return inlineChildren();
  }
};

const serializeBlockNode = (node, state, uploadedMediaByURL) => {
  if (!node) return '';
  if (node.nodeType === Node.TEXT_NODE) {
    const text = String(node.textContent || '').trim();
    return text ? escapeMarkdownText(text) : '';
  }
  if (node.nodeType !== Node.ELEMENT_NODE) {
    return '';
  }

  const tag = node.tagName.toUpperCase();
  const inlineChildren = () =>
    Array.from(node.childNodes)
      .map((child) => serializeInlineNode(child, state, uploadedMediaByURL))
      .join('')
      .trim();

  switch (tag) {
    case 'P': {
      const text = inlineChildren();
      return text ? text : '';
    }
    case 'H1':
    case 'H2':
    case 'H3':
    case 'H4':
    case 'H5':
    case 'H6': {
      const level = Number(tag.slice(1));
      const text = inlineChildren();
      return text ? `${'#'.repeat(level)} ${text}` : '';
    }
    case 'BLOCKQUOTE': {
      const inner = serializeChildrenToMarkdown(node.childNodes, state, uploadedMediaByURL);
      const lines = inner.split('\n');
      return lines.map((line) => (line.trim() ? `> ${line}` : '>')).join('\n');
    }
    case 'PRE': {
      const codeNode = node.querySelector('code') || node;
      const code = String(codeNode.textContent || '').replace(/\n$/, '');
      return `\`\`\`\n${code}\n\`\`\``;
    }
    case 'UL':
    case 'OL':
      return serializeList(node, state, uploadedMediaByURL);
    case 'HR':
      return '---';
    case 'DIV':
      return serializeChildrenToMarkdown(node.childNodes, state, uploadedMediaByURL);
    default:
      return inlineChildren();
  }
};

const serializeListItem = (node, ordered, index, state, uploadedMediaByURL) => {
  const marker = ordered ? `${index + 1}. ` : '- ';
  const nestedLists = [];
  const inlineNodes = [];
  Array.from(node.childNodes).forEach((child) => {
    if (
      child.nodeType === Node.ELEMENT_NODE &&
      ['UL', 'OL'].includes(child.tagName.toUpperCase())
    ) {
      nestedLists.push(child);
    } else {
      inlineNodes.push(child);
    }
  });

  const text = inlineNodes
    .map((child) => serializeInlineNode(child, state, uploadedMediaByURL))
    .join('')
    .trim();
  const lines = [`${marker}${text}`.trimEnd()];
  nestedLists.forEach((list) => {
    const nested = serializeList(list, state, uploadedMediaByURL, 1).trimEnd();
    if (nested) {
      lines.push(nested);
    }
  });
  return lines.join('\n');
};

const serializeList = (node, state, uploadedMediaByURL, depth = 0) => {
  const ordered = node.tagName.toUpperCase() === 'OL';
  const indent = '  '.repeat(depth);
  return Array.from(node.children)
    .filter((child) => child.tagName && child.tagName.toUpperCase() === 'LI')
    .map((item, index) =>
      serializeListItem(item, ordered, index, state, uploadedMediaByURL)
        .split('\n')
        .map((line, lineIndex) => (lineIndex === 0 ? `${indent}${line}` : `${indent}  ${line}`))
        .join('\n')
    )
    .join('\n');
};

const serializeChildrenToMarkdown = (nodes, state, uploadedMediaByURL) => {
  const blocks = [];
  Array.from(nodes).forEach((node) => {
    const serialized = serializeBlockNode(node, state, uploadedMediaByURL);
    if (serialized && String(serialized).trim()) {
      blocks.push(serialized);
    }
  });
  return blocks.join('\n\n');
};

const htmlToMarkdown = (html, state, uploadedMediaByURL) => {
  const root = document.createElement('div');
  root.innerHTML = String(html ?? '');
  return normalizeMarkdownSpacing(
    serializeChildrenToMarkdown(root.childNodes, state, uploadedMediaByURL)
  );
};

const markdownBodyToHTML = (markdown, state) => blockMarkdownToHTML(markdown, state.media || []);

const currentDocumentMediaDir = (sourcePath) => {
  const normalized = String(sourcePath || '')
    .replaceAll('\\', '/')
    .replace(/^content\//, '')
    .replace(/\.md$/i, '')
    .replace(/^\/+/, '')
    .replace(/\/+$/, '');
  return normalized;
};

export const renderQuillToolbar = (toolbarId) => `
  <div id="${toolbarId}" class="quill-toolbar">
    <span class="ql-formats">
      <button class="ql-bold" type="button"></button>
      <button class="ql-italic" type="button"></button>
      <button class="ql-strike" type="button"></button>
    </span>
    <span class="ql-formats">
      <button class="ql-header" value="1" type="button"></button>
      <button class="ql-header" value="2" type="button"></button>
      <button class="ql-blockquote" type="button"></button>
      <button class="ql-code-block" type="button"></button>
    </span>
    <span class="ql-formats">
      <button class="ql-list" value="ordered" type="button"></button>
      <button class="ql-list" value="bullet" type="button"></button>
      <button class="ql-link" type="button"></button>
      <button class="ql-image" type="button"></button>
    </span>
    <span class="ql-formats">
      <button class="ql-clean" type="button"></button>
    </span>
  </div>`;

export const renderZenModeModal = ({ state, escapeHTML, renderPreviewFrame }) => {
  if (!state.documentZenMode?.open) return '';
  const loadingText = state.documentZenMode.loading
    ? 'Loading preview and editor…'
    : 'Live preview ready';
  const loadingClass = state.documentZenMode.loading
    ? 'zen-status zen-status-loading'
    : 'zen-status zen-status-ready';
  const error = state.documentZenMode.error
    ? `<div class="zen-error">${escapeHTML(state.documentZenMode.error)}</div>`
    : '';
  return `
    <div class="zen-overlay" role="dialog" aria-modal="true" aria-label="Zen mode editor">
      <div class="zen-shell">
        <header class="zen-header">
          <div>
            <div class="zen-eyebrow">Zen Mode</div>
          </div>
          <div class="zen-header-actions">
            <span id="zen-status" class="${loadingClass}">${escapeHTML(loadingText)}</span>
            <button type="button" class="ghost" id="zen-preview-refresh">Refresh Preview</button>
            <button type="button" class="ghost" id="zen-save-document">Save Document</button>
            <button type="button" class="ghost danger" id="zen-close">Close</button>
          </div>
        </header>
        ${error}
        <div class="zen-grid">
          <section class="zen-pane zen-edit-pane">
            <div class="zen-pane-header">
              <strong>Edit</strong>
            </div>
            <div class="zen-quill-shell">
              ${renderQuillToolbar('zen-toolbar')}
              <div id="zen-editor" class="zen-editor" aria-label="Article body editor"></div>
            </div>
          </section>
          <section class="zen-pane zen-preview-pane" id="zen-preview">
            <div class="zen-pane-header">
              <strong>Preview</strong>
            </div>
            ${
              state.documentZenMode.previewHtml
                ? renderPreviewFrame?.(state.documentZenMode.previewHtml)
                : '<div class="zen-loading">Preview will appear after the first render.</div>'
            }
          </section>
        </div>
      </div>
    </div>`;
};

export const createQuillEditorController = ({
  state,
  admin,
  render,
  defaultLang,
  renderPreviewFrame,
  parseDocumentEditor,
  buildDocumentRaw,
}) => {
  const root = document.getElementById('app');
  const themeBase = `${root?.dataset.adminBase || '/__admin'}/theme`;
  const assetURLs = {
    css: `${themeBase}/vendor/quill/quill.snow.css`,
    js: `${themeBase}/vendor/quill/quill.js`,
  };

  let quillPromise = null;
  let zenPreviewTimer = null;
  let zenPreviewRequestId = 0;
  let primaryQuill = null;
  let zenQuill = null;
  let primaryMount = null;
  let zenMount = null;
  const uploadedMediaByURL = new Map();
  const pendingUploadInputs = new Map();

  const clearZenPreviewTimer = () => {
    if (zenPreviewTimer) {
      window.clearTimeout(zenPreviewTimer);
      zenPreviewTimer = null;
    }
  };

  const updateStatus = (kind, message) => {
    const id = kind === 'zen' ? 'zen-status' : 'document-quill-status';
    const node = document.getElementById(id);
    if (node) node.textContent = message;
  };

  const loadQuill = async () => {
    if (window.Quill) return window.Quill;
    if (!quillPromise) {
      quillPromise = loadStylesheet(assetURLs.css)
        .then(() => loadScript(assetURLs.js))
        .catch((error) => {
          quillPromise = null;
          throw error;
        });
    }
    return quillPromise;
  };

  const getBodyMarkdown = () => {
    const rawNode = document.getElementById('document-raw');
    const raw = String(rawNode?.value || state.documentEditor.raw || '');
    const parsed = parseDocumentEditor(raw, state.documentEditor.source_path, defaultLang);
    return parsed.body || '';
  };

  const bodyHTML = () => {
    if (state.documentEditor.html_body) {
      return state.documentEditor.html_body;
    }
    return markdownBodyToHTML(getBodyMarkdown(), state);
  };

  const rebuildRawFromQuill = (quill, kind) => {
    const rawNode = document.getElementById('document-raw');
    if (!rawNode) return;
    const parsed = parseDocumentEditor(
      rawNode.value || state.documentEditor.raw,
      state.documentEditor.source_path,
      defaultLang
    );
    const nextBody = htmlToMarkdown(quill.root.innerHTML, state, uploadedMediaByURL);
    const nextRaw = buildDocumentRaw(parsed.fields, nextBody, parsed.extraLines, defaultLang);
    rawNode.value = nextRaw;
    state.documentEditor.raw = nextRaw;
    state.documentEditor.html_body = quill.root.innerHTML;
    rawNode.dispatchEvent(new Event('input', { bubbles: true }));
    if (kind === 'zen' && state.documentZenMode?.open) {
      queuePreviewRefresh();
    }
  };

  const updatePreviewPane = (html) => {
    const pane = document.getElementById('zen-preview');
    if (!pane) return;
    pane.innerHTML = `
      <div class="zen-pane-header">
        <strong>Preview</strong>
      </div>
      ${html ? renderPreviewFrame(html) : '<div class="zen-loading">Preview will appear after the first render.</div>'}`;
  };

  const refreshPreview = async () => {
    if (!state.documentZenMode?.open) return;
    const requestId = ++zenPreviewRequestId;
    state.documentZenMode.loading = true;
    updateStatus('zen', 'Updating preview…');
    try {
      const preview = await admin.documents.preview({
        source_path: state.documentEditor.source_path,
        raw: document.getElementById('document-raw')?.value || state.documentEditor.raw,
        fields: state.documentFieldValues,
      });
      if (requestId !== zenPreviewRequestId || !state.documentZenMode?.open) return;
      state.documentPreview = preview;
      state.documentZenMode.previewHtml = preview.html || '';
      state.documentZenMode.loading = false;
      state.documentZenMode.error = '';
      updatePreviewPane(preview.html || '');
      updateStatus('zen', 'Preview synced');
    } catch (error) {
      if (requestId !== zenPreviewRequestId || !state.documentZenMode?.open) return;
      state.documentZenMode.loading = false;
      state.documentZenMode.error = error.message || String(error);
      updateStatus('zen', 'Preview failed');
      render();
    }
  };

  const queuePreviewRefresh = () => {
    clearZenPreviewTimer();
    zenPreviewTimer = window.setTimeout(() => {
      void refreshPreview();
    }, 1000);
    updateStatus('zen', 'Preview sync queued');
  };

  const getImageUploadDir = () => currentDocumentMediaDir(state.documentEditor.source_path);

  const uploadImageFile = async (file) => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('collection', 'images');
    const dir = getImageUploadDir();
    if (dir) {
      formData.append('dir', dir);
    }
    const uploaded = await admin.media.upload(formData);
    if (uploaded?.public_url && uploaded?.reference) {
      uploadedMediaByURL.set(uploaded.public_url, uploaded.reference);
    }
    return uploaded;
  };

  const findInsertedImageNode = (quill, publicURL) =>
    [...quill.root.querySelectorAll('img')].find((img) => img.getAttribute('src') === publicURL) ||
    null;

  const insertUploadedImage = async (quill, file) => {
    const uploaded = await uploadImageFile(file);
    const range = quill.getSelection(true) || { index: quill.getLength(), length: 0 };
    const alt = String(
      uploaded?.metadata?.alt || uploaded?.metadata?.title || file.name.replace(/\.[^.]+$/, '')
    ).trim();
    quill.insertEmbed(range.index, 'image', uploaded.public_url, 'user');
    quill.insertText(range.index + 1, '\n', 'user');
    quill.setSelection(range.index + 2, 0, 'silent');
    const imageNode = findInsertedImageNode(quill, uploaded.public_url);
    if (imageNode) {
      imageNode.setAttribute('alt', alt);
    }
    updateStatus(quill === zenQuill ? 'zen' : 'primary', `Uploaded ${file.name}`);
    return uploaded;
  };

  const handleImageFiles = async (quill, files) => {
    for (const file of files) {
      if (!file || !String(file.type || '').startsWith('image/')) continue;
      await insertUploadedImage(quill, file);
      rebuildRawFromQuill(quill, quill === zenQuill ? 'zen' : 'primary');
    }
  };

  const makeFilePicker = (kind) => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = 'image/*';
    input.multiple = true;
    input.hidden = true;
    input.dataset.quillImagePicker = kind;
    document.body.appendChild(input);
    pendingUploadInputs.set(kind, input);
    input.addEventListener('change', async () => {
      const quill = kind === 'zen' ? zenQuill : primaryQuill;
      if (!quill) return;
      try {
        await handleImageFiles(quill, Array.from(input.files || []));
      } catch (error) {
        state.error = error.message || String(error);
        render();
      } finally {
        input.value = '';
      }
    });
    return input;
  };

  const requestImageUpload = async (kind, quill) => {
    let input = pendingUploadInputs.get(kind);
    if (!input || !document.body.contains(input)) {
      input = makeFilePicker(kind);
    }
    quill.focus();
    input.click();
  };

  const bindQuillInteractions = (quill, kind) => {
    const toolbar = quill.getModule('toolbar');
    toolbar?.addHandler('image', () => {
      void requestImageUpload(kind, quill);
    });

    quill.root.addEventListener('paste', async (event) => {
      const files = Array.from(event.clipboardData?.files || []).filter((file) =>
        String(file.type || '').startsWith('image/')
      );
      if (!files.length) return;
      event.preventDefault();
      try {
        await handleImageFiles(quill, files);
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });

    quill.root.addEventListener('drop', async (event) => {
      const files = Array.from(event.dataTransfer?.files || []).filter((file) =>
        String(file.type || '').startsWith('image/')
      );
      if (!files.length) return;
      event.preventDefault();
      try {
        await handleImageFiles(quill, files);
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });

    quill.on('text-change', () => {
      rebuildRawFromQuill(quill, kind);
    });
  };

  const mountEditor = async (kind, mountId, toolbarId, initialHTML, placeholder) => {
    const mount = document.getElementById(mountId);
    const toolbar = document.getElementById(toolbarId);
    if (!mount || !toolbar) return null;
    const currentMount = kind === 'zen' ? zenMount : primaryMount;
    const currentQuill = kind === 'zen' ? zenQuill : primaryQuill;
    if (currentMount === mount && currentQuill) return currentQuill;

    if (kind === 'zen') {
      zenMount = mount;
      zenQuill = null;
    } else {
      primaryMount = mount;
      primaryQuill = null;
    }

    mount.innerHTML = '';
    try {
      const Quill = await loadQuill();
      const quill = new Quill(mount, {
        theme: 'snow',
        placeholder,
        modules: {
          toolbar: `#${toolbarId}`,
          history: {
            delay: 500,
          },
        },
      });
      bindQuillInteractions(quill, kind);
      const html = String(initialHTML || '').trim() ? initialHTML : '<p><br></p>';
      const delta = quill.clipboard.convert({ html: `${html}<p><br></p>`, text: '\n' });
      quill.setContents(delta, 'silent');
      if (kind === 'zen') {
        zenQuill = quill;
        state.documentZenMode.loading = false;
        updateStatus('zen', 'Live preview ready');
      } else {
        primaryQuill = quill;
        updateStatus('primary', 'Editor ready.');
      }
      return quill;
    } catch (error) {
      const message = error.message || String(error);
      if (kind === 'zen') {
        state.documentZenMode.error = message;
        updateStatus('zen', 'Quill unavailable');
      } else {
        state.error = message;
      }
      mount.innerHTML = `<div class="${kind === 'zen' ? 'zen-error' : 'quill-error'}">Failed to load Quill: ${escapeHTML(message)}</div>`;
      render();
      return null;
    }
  };

  const mountPrimaryEditor = async () => {
    const initialHTML = bodyHTML();
    await mountEditor(
      'primary',
      'document-quill-editor',
      'document-quill-toolbar',
      initialHTML,
      'Write the article body here.'
    );
  };

  const mountZenEditor = async () => {
    if (!state.documentZenMode?.open) return;
    const initialHTML = bodyHTML();
    await mountEditor(
      'zen',
      'zen-editor',
      'zen-toolbar',
      initialHTML,
      'Write the article body here.'
    );
  };

  const open = async () => {
    if (state.documentZenMode?.open) return;
    state.documentZenMode = {
      open: true,
      loading: true,
      error: '',
      previewHtml: '',
    };
    render();
    await refreshPreview();
    render();
    await mountZenEditor();
  };

  const close = () => {
    zenPreviewRequestId += 1;
    clearZenPreviewTimer();
    zenQuill = null;
    zenMount = null;
    state.documentZenMode = {
      open: false,
      loading: false,
      error: '',
      previewHtml: '',
    };
    render();
  };

  const save = () => {
    document.getElementById('document-save-form')?.requestSubmit();
  };

  const disposeEditor = () => {
    clearZenPreviewTimer();
    zenQuill = null;
    zenMount = null;
  };

  return {
    loadQuill,
    mountPrimaryEditor,
    mountEditor: mountZenEditor,
    open,
    close,
    save,
    refreshPreview,
    queuePreviewRefresh,
    disposeEditor,
  };
};
