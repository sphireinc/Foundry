export const createEditorSync = ({
  state,
  defaultLang,
  parseTagInput,
  inferLangFromSourcePath,
  parseDocumentEditor,
  buildDocumentRaw,
}) => {
  const editorElements = () => ({
    sourcePath: document.getElementById('document-source-path'),
    versionComment: document.getElementById('document-version-comment'),
    raw: document.getElementById('document-raw'),
    title: document.getElementById('document-frontmatter-title'),
    slug: document.getElementById('document-frontmatter-slug'),
    layout: document.getElementById('document-frontmatter-layout'),
    date: document.getElementById('document-frontmatter-date'),
    summary: document.getElementById('document-frontmatter-summary'),
    tags: document.getElementById('document-frontmatter-tags'),
    categories: document.getElementById('document-frontmatter-categories'),
    draft: document.getElementById('document-frontmatter-draft'),
    archived: document.getElementById('document-frontmatter-archived'),
    lang: document.getElementById('document-frontmatter-lang'),
    workflow: document.getElementById('document-frontmatter-workflow'),
    scheduledPublishAt: document.getElementById('document-frontmatter-scheduled-publish-at'),
    scheduledUnpublishAt: document.getElementById('document-frontmatter-scheduled-unpublish-at'),
    editorialNote: document.getElementById('document-frontmatter-editorial-note'),
  });

  const setStructuredFields = (parsed) => {
    const elements = editorElements();
    if (!elements.raw) return;
    elements.title && (elements.title.value = parsed.fields.title || '');
    elements.slug && (elements.slug.value = parsed.fields.slug || '');
    elements.layout && (elements.layout.value = parsed.fields.layout || 'post');
    elements.date && (elements.date.value = parsed.fields.date || '');
    elements.summary && (elements.summary.value = parsed.fields.summary || '');
    elements.tags && (elements.tags.value = (parsed.fields.tags || []).join(', '));
    elements.categories &&
      (elements.categories.value = (parsed.fields.categories || []).join(', '));
    elements.draft && (elements.draft.checked = !!parsed.fields.draft);
    elements.archived && (elements.archived.checked = !!parsed.fields.archived);
    elements.lang &&
      (elements.lang.value =
        parsed.fields.lang ||
        inferLangFromSourcePath(
          elements.sourcePath?.value || state.documentEditor.source_path,
          defaultLang
        ));
    elements.workflow && (elements.workflow.value = parsed.fields.workflow || 'draft');
    elements.scheduledPublishAt &&
      (elements.scheduledPublishAt.value = parsed.fields.scheduled_publish_at || '');
    elements.scheduledUnpublishAt &&
      (elements.scheduledUnpublishAt.value = parsed.fields.scheduled_unpublish_at || '');
    elements.editorialNote && (elements.editorialNote.value = parsed.fields.editorial_note || '');
  };

  const syncStructuredEditorFromRaw = () => {
    const elements = editorElements();
    if (!elements.raw) return;
    state.documentEditor.raw = elements.raw.value;
    if (elements.sourcePath) {
      state.documentEditor.source_path = elements.sourcePath.value;
    }
    if (elements.versionComment) {
      state.documentEditor.version_comment = elements.versionComment.value;
    }
    setStructuredFields(
      parseDocumentEditor(
        elements.raw.value,
        elements.sourcePath?.value || state.documentEditor.source_path,
        defaultLang
      )
    );
  };

  const syncRawFromStructuredEditor = () => {
    const elements = editorElements();
    if (!elements.raw) return;
    const parsed = parseDocumentEditor(
      elements.raw.value,
      elements.sourcePath?.value || state.documentEditor.source_path,
      defaultLang
    );
    const nextFields = {
      title: elements.title?.value || '',
      slug: elements.slug?.value || '',
      layout: elements.layout?.value || parsed.fields.layout || 'post',
      date: elements.date?.value || '',
      summary: elements.summary?.value || '',
      tags: parseTagInput(elements.tags?.value || ''),
      categories: parseTagInput(elements.categories?.value || ''),
      draft: !!elements.draft?.checked,
      archived: !!elements.archived?.checked,
      lang: elements.lang?.value || defaultLang,
      workflow: elements.workflow?.value || parsed.fields.workflow || 'draft',
      scheduled_publish_at: elements.scheduledPublishAt?.value || '',
      scheduled_unpublish_at: elements.scheduledUnpublishAt?.value || '',
      editorial_note: elements.editorialNote?.value || '',
    };
    if (nextFields.workflow === 'archived') {
      nextFields.archived = true;
      nextFields.draft = true;
    } else if (nextFields.workflow === 'draft') {
      nextFields.draft = true;
      nextFields.archived = false;
    } else if (nextFields.workflow === 'published') {
      nextFields.draft = false;
      nextFields.archived = false;
    } else if (nextFields.workflow === 'in_review' || nextFields.workflow === 'scheduled') {
      nextFields.draft = true;
      nextFields.archived = false;
    }
    const rebuilt = buildDocumentRaw(nextFields, parsed.body, parsed.extraLines, defaultLang);
    elements.raw.value = rebuilt;
    state.documentEditor.raw = rebuilt;
    if (elements.sourcePath) state.documentEditor.source_path = elements.sourcePath.value;
    if (elements.versionComment)
      state.documentEditor.version_comment = elements.versionComment.value;
  };

  const insertIntoMarkdown = (snippet) => {
    const elements = editorElements();
    if (!elements.raw) return;
    const textarea = elements.raw;
    const start = textarea.selectionStart ?? textarea.value.length;
    const end = textarea.selectionEnd ?? textarea.value.length;
    const prefix = start > 0 && !textarea.value.slice(0, start).endsWith('\n') ? '\n' : '';
    const suffix =
      end < textarea.value.length && !textarea.value.slice(end).startsWith('\n') ? '\n' : '';
    const nextValue =
      textarea.value.slice(0, start) + prefix + snippet + suffix + textarea.value.slice(end);
    textarea.value = nextValue;
    state.documentEditor.raw = nextValue;
    textarea.focus();
    const cursor = start + prefix.length + snippet.length;
    textarea.setSelectionRange(cursor, cursor);
    syncStructuredEditorFromRaw();
  };

  const mediaSnippet = (item, mode = 'auto') => {
    if (!item) return '';
    const label = item.metadata?.alt || item.metadata?.title || item.name;
    if (mode === 'link') {
      return `[${label}](${item.reference})`;
    }
    return item.kind === 'file'
      ? `[${label}](${item.reference})`
      : `![${label}](${item.reference})`;
  };

  const renderPreviewFrame = (html) => {
    const srcdoc =
      '<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><base target="_blank"></head><body>' +
      (html || '') +
      '</body></html>';
    return `<iframe class="preview-frame" sandbox="" referrerpolicy="no-referrer" srcdoc="${String(
      srcdoc
    )
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#39;')}"></iframe>`;
  };

  return {
    editorElements,
    setStructuredFields,
    syncStructuredEditorFromRaw,
    syncRawFromStructuredEditor,
    insertIntoMarkdown,
    mediaSnippet,
    renderPreviewFrame,
  };
};
