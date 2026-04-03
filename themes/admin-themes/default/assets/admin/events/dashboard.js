// bindDashboardEvents attaches all DOM event handlers for the current admin
// render pass.
//
// The default admin shell renders HTML strings, then rebinds event handlers
// against the new DOM. This module keeps that wiring in one place instead of
// scattering imperative listeners across views.
export const bindDashboardEvents = (ctx) => {
  const {
    root,
    state,
    admin,
    render,
    navigate,
    updateTablePage,
    releaseCurrentDocumentLock,
    syncStructuredEditorFromRaw,
    compareSnapshot,
    syncRawFromStructuredEditor,
    slugify,
    updateDocumentFieldValue,
    updateSharedFieldValue,
    getValueAtPath,
    defaultValueForSchema,
    removeNestedFieldValue,
    insertIntoMarkdown,
    mediaSnippet,
    loadDocumentIntoEditor,
    setFlash,
    fetchAll,
    loadDocumentHistory,
    loadDocumentDiff,
    resetDocumentEditor,
    snapshotValue,
    loadMediaDetail,
    loadMediaHistory,
    toggleSelection,
    setUserForm,
    resetUserForm,
    resetUserSecurity,
    selectedUserRecord,
    parseTagInput,
    parseDocumentEditor,
    buildDocumentRaw,
  } = ctx;

  const pluginRecordByName = (name) =>
    (state.plugins || []).find((item) => item.name === name) || null;
  const isRiskAckError = (error) => {
    const message = String(error?.message || error || '').toLowerCase();
    return (
      message.includes('approve_risk') ||
      message.includes('requires explicit risk approval') ||
      message.includes('acknowledge_mismatches')
    );
  };
  const confirmPluginRisk = (action, pluginName, pluginRecord = null) => {
    const lines = [
      `${action} plugin "${pluginName}"?`,
      '',
      pluginRecord?.risk_tier
        ? `Declared risk tier: ${pluginRecord.risk_tier}`
        : 'This plugin requires explicit approval before continuing.',
    ];
    if (pluginRecord?.runtime_summary?.length) {
      lines.push(`Runtime: ${pluginRecord.runtime_summary.join(' • ')}`);
    }
    if (pluginRecord?.security_mismatches?.length) {
      lines.push('');
      lines.push(`Detected security mismatches: ${pluginRecord.security_mismatches.length}`);
      lines.push(...pluginRecord.security_mismatches.slice(0, 3).map((diag) => `- ${diag.message}`));
    } else {
      lines.push('');
      lines.push('Proceeding will explicitly approve the plugin risk profile.');
    }
    lines.push('');
    lines.push('Continue?');
    return window.confirm(lines.join('\n'));
  };

  root.querySelectorAll('[data-section]').forEach((element) => {
    element.addEventListener('click', (event) => {
      event.preventDefault();
      navigate(element.dataset.section);
    });
  });

  root.querySelectorAll('[data-table-sort]').forEach((select) => {
    select.addEventListener('change', () => {
      state.tables[select.dataset.tableSort].sort = select.value;
      updateTablePage(select.dataset.tableSort, 1);
      render();
    });
  });

  root.querySelectorAll('[data-table-dir]').forEach((button) => {
    button.addEventListener('click', () => {
      const table = state.tables[button.dataset.tableDir];
      table.dir = table.dir === 'asc' ? 'desc' : 'asc';
      render();
    });
  });

  root.querySelectorAll('[data-table-page]').forEach((button) => {
    button.addEventListener('click', () => {
      const [name, step] = button.dataset.tablePage.split('|');
      const next = state.tables[name].page + (step === 'next' ? 1 : -1);
      updateTablePage(name, next);
      render();
    });
  });

  document.getElementById('logout')?.addEventListener('click', async () => {
    await releaseCurrentDocumentLock();
    try {
      await admin.session.logout();
    } catch (_error) {}
    state.session = null;
    state.flash = '';
    state.error = '';
    render();
  });

  document.getElementById('document-source-path')?.addEventListener('input', () => {
    state.documentEditor.source_path = document.getElementById('document-source-path').value;
    syncStructuredEditorFromRaw();
    compareSnapshot('document', {
      editor: state.documentEditor,
      fields: state.documentFieldValues,
      meta: state.documentMeta,
    });
  });

  document.getElementById('document-version-comment')?.addEventListener('input', () => {
    state.documentEditor.version_comment = document.getElementById(
      'document-version-comment'
    ).value;
    compareSnapshot('document', {
      editor: state.documentEditor,
      fields: state.documentFieldValues,
      meta: state.documentMeta,
    });
  });

  document.getElementById('document-raw')?.addEventListener('input', () => {
    syncStructuredEditorFromRaw();
    compareSnapshot('document', {
      editor: state.documentEditor,
      fields: state.documentFieldValues,
      meta: state.documentMeta,
    });
  });

  [
    'document-frontmatter-title',
    'document-frontmatter-slug',
    'document-frontmatter-layout',
    'document-frontmatter-date',
    'document-frontmatter-summary',
    'document-frontmatter-tags',
    'document-frontmatter-categories',
    'document-frontmatter-draft',
    'document-frontmatter-archived',
    'document-frontmatter-lang',
    'document-frontmatter-workflow',
    'document-frontmatter-scheduled-publish-at',
    'document-frontmatter-scheduled-unpublish-at',
    'document-frontmatter-editorial-note',
  ].forEach((id) => {
    const field = document.getElementById(id);
    if (!field) return;
    field.addEventListener(field.type === 'checkbox' ? 'change' : 'input', () => {
      if (id === 'document-frontmatter-title') {
        const slugField = document.getElementById('document-frontmatter-slug');
        if (slugField && !slugField.value.trim()) {
          slugField.value = slugify(field.value);
        }
      }
      syncRawFromStructuredEditor();
      compareSnapshot('document', {
        editor: state.documentEditor,
        fields: state.documentFieldValues,
        meta: state.documentMeta,
      });
    });
  });

  root.querySelectorAll('[data-custom-field]').forEach((field) => {
    const eventName = field.type === 'checkbox' ? 'change' : 'input';
    field.addEventListener(eventName, () => {
      const path = String(field.dataset.customField || '')
        .split('.')
        .filter(Boolean)
        .map((segment) => (/^\d+$/.test(segment) ? Number(segment) : segment));
      let nextValue;
      switch (field.dataset.customType) {
        case 'bool':
          nextValue = !!field.checked;
          break;
        case 'number':
          nextValue = field.value === '' ? '' : Number(field.value);
          break;
        default:
          nextValue = field.value;
          break;
      }
      updateDocumentFieldValue(path, nextValue);
    });
  });

  root.querySelectorAll('[data-repeater-add]').forEach((button) => {
    button.addEventListener('click', () => {
      const path = String(button.dataset.repeaterAdd || '')
        .split('.')
        .filter(Boolean)
        .map((segment) => (/^\d+$/.test(segment) ? Number(segment) : segment));
      const schema = state.documentFieldSchema.find((entry) => entry.name === path[0]);
      if (!schema) return;
      const current = getValueAtPath(state.documentFieldValues, path);
      const nextItems = Array.isArray(current) ? [...current] : [];
      nextItems.push(defaultValueForSchema(schema.item));
      updateDocumentFieldValue(path, nextItems);
    });
  });

  root.querySelectorAll('[data-repeater-remove]').forEach((button) => {
    button.addEventListener('click', () => {
      const path = String(button.dataset.repeaterRemove || '')
        .split('.')
        .filter(Boolean)
        .map((segment) => (/^\d+$/.test(segment) ? Number(segment) : segment));
      removeNestedFieldValue(state, path);
      compareSnapshot('document', {
        editor: state.documentEditor,
        fields: state.documentFieldValues,
        meta: state.documentMeta,
      });
      render();
    });
  });

  root.querySelectorAll('[data-shared-custom-field]').forEach((field) => {
    const eventName = field.type === 'checkbox' ? 'change' : 'input';
    field.addEventListener(eventName, () => {
      const path = String(field.dataset.sharedCustomField || '')
        .split('.')
        .filter(Boolean)
        .map((segment) => (/^\d+$/.test(segment) ? Number(segment) : segment));
      let nextValue;
      switch (field.dataset.customType) {
        case 'bool':
          nextValue = !!field.checked;
          break;
        case 'number':
          nextValue = field.value === '' ? '' : Number(field.value);
          break;
        default:
          nextValue = field.value;
          break;
      }
      updateSharedFieldValue(path, nextValue);
    });
  });

  root.querySelectorAll('[data-shared-repeater-add]').forEach((button) => {
    button.addEventListener('click', () => {
      const path = String(button.dataset.sharedRepeaterAdd || '')
        .split('.')
        .filter(Boolean)
        .map((segment) => (/^\d+$/.test(segment) ? Number(segment) : segment));
      const contractKey = path[0];
      const contract = (state.sharedFieldContracts || []).find((entry) => entry.key === contractKey);
      const schema = (contract?.fields || []).find((entry) => entry.name === path[1]);
      if (!schema) return;
      const current = getValueAtPath(state.customFields?.values || {}, path);
      const nextItems = Array.isArray(current) ? [...current] : [];
      nextItems.push(defaultValueForSchema(schema.item));
      updateSharedFieldValue(path, nextItems);
    });
  });

  root.querySelectorAll('[data-shared-repeater-remove]').forEach((button) => {
    button.addEventListener('click', () => {
      const path = String(button.dataset.sharedRepeaterRemove || '')
        .split('.')
        .filter(Boolean)
        .map((segment) => (/^\d+$/.test(segment) ? Number(segment) : segment));
      const wrapper = { documentFieldValues: state.customFields?.values || {} };
      removeNestedFieldValue(wrapper, path);
      state.customFields = state.customFields || { path: 'content/custom-fields.yaml', raw: '', values: {} };
      state.customFields.values = wrapper.documentFieldValues;
      compareSnapshot('customFields', state.customFields.values || {});
      render();
    });
  });

  document.getElementById('document-media-picker-query')?.addEventListener('input', (event) => {
    state.mediaPickerQuery = event.target.value;
    render();
  });

  root.querySelectorAll('[data-insert-media]').forEach((button) => {
    button.addEventListener('click', () => {
      const item = state.media.find((entry) => entry.reference === button.dataset.insertMedia);
      if (!item) return;
      insertIntoMarkdown(mediaSnippet(item, button.dataset.insertMode || 'auto'));
    });
  });

  document.getElementById('document-create-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    try {
      const created = await admin.documents.create({
        kind: document.getElementById('document-create-kind').value,
        slug: document.getElementById('document-create-slug').value,
        lang: document.getElementById('document-create-lang').value,
        archetype: document.getElementById('document-create-archetype').value,
      });
      const detail = await admin.documents
        .get(created.source_path, { include_drafts: 1 })
        .catch(() => ({
          source_path: created.source_path,
          raw_body: created.raw || '',
          lock: null,
          field_schema: [],
          fields: {},
        }));
      await loadDocumentIntoEditor(detail);
      setFlash('Document created.');
      await fetchAll(false);
      navigate('editor');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('document-search-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    state.documentQuery = document.getElementById('document-search-query').value.trim();
    state.documentFilters.status = document.getElementById('document-filter-status')?.value || '';
    state.documentFilters.type = document.getElementById('document-filter-type')?.value || '';
    state.documentFilters.lang = document.getElementById('document-filter-lang')?.value || '';
    state.documentFilters.author = document.getElementById('document-filter-author')?.value || '';
    state.documentFilters.tag = document.getElementById('document-filter-tag')?.value || '';
    state.documentFilters.category =
      document.getElementById('document-filter-category')?.value || '';
    state.documentFilters.dateFrom =
      document.getElementById('document-filter-date-from')?.value || '';
    state.documentFilters.dateTo = document.getElementById('document-filter-date-to')?.value || '';
    await fetchAll();
    navigate('documents');
  });

  document.getElementById('document-search-clear')?.addEventListener('click', async () => {
    state.documentQuery = '';
    state.documentFilters = {
      status: '',
      type: '',
      lang: '',
      author: '',
      tag: '',
      category: '',
      dateFrom: '',
      dateTo: '',
    };
    const input = document.getElementById('document-search-query');
    if (input) input.value = '';
    await fetchAll();
    navigate('documents');
  });

  root.querySelectorAll('[data-open-editor]').forEach((button) => {
    button.addEventListener('click', () => {
      navigate('editor');
    });
  });

  root.querySelectorAll('[data-open-documents]').forEach((button) => {
    button.addEventListener('click', () => {
      navigate('documents');
    });
  });

  document.getElementById('editor-preview-documents')?.addEventListener('click', async () => {
    try {
      state.documentPreview = await admin.documents.preview({
        source_path:
          document.getElementById('document-source-path')?.value ||
          state.documentEditor.source_path,
        raw: document.getElementById('document-raw')?.value || state.documentEditor.raw,
        fields: state.documentFieldValues,
      });
      setFlash('Preview rendered.');
      navigate('documents');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('document-save-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    try {
      const saved = await admin.documents.save({
        source_path: document.getElementById('document-source-path').value,
        raw: document.getElementById('document-raw').value,
        fields: state.documentFieldValues,
        version_comment: document.getElementById('document-version-comment').value,
        lock_token: state.documentEditor.lock_token,
      });
      state.documentEditor = {
        source_path: saved.source_path || document.getElementById('document-source-path').value,
        raw: saved.raw || document.getElementById('document-raw').value,
        version_comment: '',
        lock_token: state.documentEditor.lock_token,
      };
      const detail = await admin.documents.get(state.documentEditor.source_path, {
        include_drafts: 1,
      });
      state.documentFieldSchema = detail.field_schema || [];
      state.documentFieldValues = ctx.clone(detail.fields || {});
      state.documentMeta = {
        status: detail.status || 'draft',
        author: detail.author || '',
        last_editor: detail.last_editor || '',
        created_at: detail.created_at || '',
        updated_at: detail.updated_at || '',
      };
      snapshotValue('document', {
        editor: state.documentEditor,
        fields: state.documentFieldValues,
        meta: state.documentMeta,
      });
      setFlash('Document saved.');
      await fetchAll(false);
      navigate('editor');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('document-preview-button')?.addEventListener('click', async () => {
    try {
      state.documentPreview = await admin.documents.preview({
        source_path: document.getElementById('document-source-path').value,
        raw: document.getElementById('document-raw').value,
        fields: state.documentFieldValues,
      });
      setFlash('Preview rendered.');
      navigate('documents');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('document-reset-button')?.addEventListener('click', () => {
    releaseCurrentDocumentLock().finally(() => {
      resetDocumentEditor();
      snapshotValue('document', {
        editor: state.documentEditor,
        fields: state.documentFieldValues,
        meta: state.documentMeta,
      });
      setFlash('Editor reset.');
      render();
    });
  });

  root.querySelectorAll('[data-apply-workflow]').forEach((button) => {
    button.addEventListener('click', () => {
      const workflowField = document.getElementById('document-frontmatter-workflow');
      if (workflowField) {
        workflowField.value = button.dataset.applyWorkflow;
        syncRawFromStructuredEditor();
        compareSnapshot('document', {
          editor: state.documentEditor,
          fields: state.documentFieldValues,
          meta: state.documentMeta,
        });
        render();
      }
    });
  });

  root.querySelectorAll('[data-edit-document]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        const detail = await admin.documents.get(button.dataset.editDocument, {
          include_drafts: 1,
        });
        await loadDocumentIntoEditor(detail);
        setFlash('Document loaded.');
        navigate('editor');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-history-document]').forEach((button) => {
    button.addEventListener('click', async () => {
      await loadDocumentHistory(button.dataset.historyDocument);
    });
  });

  root.querySelectorAll('[data-set-document-status]').forEach((button) => {
    button.addEventListener('click', async () => {
      const [sourcePath, status] = button.dataset.setDocumentStatus.split('|');
      try {
        await admin.documents.setStatus({
          source_path: sourcePath,
          status,
          scheduled_publish_at:
            document.getElementById('document-frontmatter-scheduled-publish-at')?.value || '',
          scheduled_unpublish_at:
            document.getElementById('document-frontmatter-scheduled-unpublish-at')?.value || '',
          editorial_note:
            document.getElementById('document-frontmatter-editorial-note')?.value || '',
          lock_token:
            state.documentEditor.source_path === sourcePath ? state.documentEditor.lock_token : '',
        });
        setFlash(`Document moved to ${status}.`);
        if (state.documentEditor.source_path === sourcePath) {
          const detail = await admin.documents.get(sourcePath, { include_drafts: 1 });
          await loadDocumentIntoEditor(detail);
        }
        await fetchAll(false);
        navigate(state.documentEditor.source_path === sourcePath ? 'editor' : 'documents');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-restore-document]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        if (!window.confirm(`Restore ${button.dataset.restoreDocument} as the current document?`))
          return;
        const restored = await admin.documents.restore({ path: button.dataset.restoreDocument });
        setFlash('Document restored.');
        await fetchAll(false);
        await loadDocumentHistory(restored.restored_path || restored.path, false);
        const detail = await admin.documents.get(restored.restored_path || restored.path, {
          include_drafts: 1,
        });
        await loadDocumentIntoEditor(detail);
        navigate('editor');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-purge-document]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        if (
          !window.confirm(
            `Permanently purge ${button.dataset.purgeDocument}? This cannot be undone.`
          )
        )
          return;
        await admin.documents.purge({ path: button.dataset.purgeDocument });
        setFlash('Document version purged.');
        await fetchAll(false);
        if (state.documentHistoryPath) {
          await loadDocumentHistory(state.documentHistoryPath, false);
        }
        navigate('history');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-preview-restore-document]').forEach((button) => {
    button.addEventListener('click', async () => {
      const currentPath = state.documentHistory.find((entry) => entry.state === 'current')?.path;
      if (!currentPath) {
        state.error = 'Load document history before requesting a diff.';
        render();
        return;
      }
      await loadDocumentDiff(button.dataset.previewRestoreDocument, currentPath);
      setFlash('Restore preview loaded. Review the diff before restoring.');
      navigate('history');
    });
  });

  root.querySelectorAll('[data-diff-mode]').forEach((button) => {
    button.addEventListener('click', () => {
      state.documentDiffMode = button.dataset.diffMode;
      render();
    });
  });

  root.querySelectorAll('[data-delete-document]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        if (!window.confirm(`Move ${button.dataset.deleteDocument} to trash?`)) return;
        await admin.documents.delete({
          source_path: button.dataset.deleteDocument,
          lock_token:
            state.documentEditor.source_path === button.dataset.deleteDocument
              ? state.documentEditor.lock_token
              : '',
        });
        await releaseCurrentDocumentLock();
        resetDocumentEditor();
        setFlash('Document moved to trash.');
        await fetchAll(false);
        navigate('trash');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  document.getElementById('media-upload-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const file = document.getElementById('media-file').files[0];
    if (!file) {
      state.error = 'Choose a file to upload.';
      render();
      return;
    }
    try {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('collection', document.getElementById('media-collection').value);
      const uploaded = await admin.media.upload(formData);
      state.selectedMediaReference = uploaded.reference;
      setFlash('Media uploaded.');
      await fetchAll(false);
      await loadMediaDetail(uploaded.reference, false);
      navigate('media');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('media-search-apply')?.addEventListener('click', async () => {
    state.mediaQuery = document.getElementById('media-search-query').value.trim();
    state.mediaFilters.kind = document.getElementById('media-filter-kind')?.value || '';
    state.mediaFilters.collection = document.getElementById('media-filter-collection')?.value || '';
    state.mediaFilters.usage = document.getElementById('media-filter-usage')?.value || '';
    await fetchAll();
    navigate('media');
  });

  document.getElementById('media-search-clear')?.addEventListener('click', async () => {
    state.mediaQuery = '';
    state.mediaFilters = { kind: '', collection: '', usage: '' };
    const input = document.getElementById('media-search-query');
    if (input) input.value = '';
    await fetchAll();
    navigate('media');
  });

  root.querySelectorAll('[data-edit-media]').forEach((button) => {
    button.addEventListener('click', async () => {
      await loadMediaDetail(button.dataset.editMedia);
    });
  });

  root.querySelectorAll('[data-prepare-media-replace]').forEach((button) => {
    button.addEventListener('click', async () => {
      await loadMediaDetail(button.dataset.prepareMediaReplace);
    });
  });

  root.querySelectorAll('[data-history-media-path]').forEach((button) => {
    button.addEventListener('click', async () => {
      await loadMediaHistory(button.dataset.historyMediaPath);
    });
  });

  document.getElementById('media-metadata-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    if (!state.selectedMediaReference) return;
    try {
      state.mediaDetail = await admin.media.updateMetadata({
        reference: state.selectedMediaReference,
        version_comment: document.getElementById('media-version-comment').value,
        metadata: {
          title: document.getElementById('media-title').value,
          alt: document.getElementById('media-alt').value,
          caption: document.getElementById('media-caption').value,
          description: document.getElementById('media-description').value,
          credit: document.getElementById('media-credit').value,
          focal_x: document.getElementById('media-focal-x').value,
          focal_y: document.getElementById('media-focal-y').value,
          tags: document
            .getElementById('media-tags')
            .value.split(',')
            .map((tag) => tag.trim())
            .filter(Boolean),
        },
      });
      state.mediaVersionComment = '';
      snapshotValue('media', {
        reference: state.selectedMediaReference,
        metadata: state.mediaDetail.metadata,
        versionComment: '',
      });
      setFlash('Media metadata saved.');
      await fetchAll(false);
      navigate('media');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('media-replace-button')?.addEventListener('click', async () => {
    if (!state.selectedMediaReference) return;
    const file = document.getElementById('media-replace-file').files[0];
    if (!file) {
      state.error = 'Choose a file to replace the current media.';
      render();
      return;
    }
    try {
      const formData = new FormData();
      formData.append('reference', state.selectedMediaReference);
      formData.append('file', file);
      await admin.media.replace(formData);
      setFlash('Media replaced.');
      await fetchAll(false);
      await loadMediaDetail(state.selectedMediaReference, false);
      navigate('media');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  root.querySelectorAll('[data-delete-media]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        if (!window.confirm(`Move ${button.dataset.deleteMedia} to trash?`)) return;
        await admin.media.delete({ reference: button.dataset.deleteMedia });
        if (state.selectedMediaReference === button.dataset.deleteMedia) {
          state.selectedMediaReference = '';
          state.mediaDetail = null;
        }
        setFlash('Media deleted.');
        await fetchAll(false);
        navigate('trash');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-restore-media-path]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        if (
          !window.confirm(`Restore ${button.dataset.restoreMediaPath} as the current media file?`)
        )
          return;
        const restored = await admin.media.restore({ path: button.dataset.restoreMediaPath });
        setFlash('Media restored.');
        await fetchAll(false);
        if (restored.restored_path) {
          const restoredMedia = state.media.find(
            (item) =>
              item.path && `content/${item.collection}/${item.path}` === restored.restored_path
          );
          if (restoredMedia) {
            await loadMediaHistory(
              `content/${restoredMedia.collection}/${restoredMedia.path}`,
              false
            );
          } else {
            state.mediaHistory = [];
            state.mediaHistoryReference = '';
          }
        }
        navigate('history');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-purge-media-path]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        if (
          !window.confirm(
            `Permanently purge ${button.dataset.purgeMediaPath}? This cannot be undone.`
          )
        )
          return;
        await admin.media.purge({ path: button.dataset.purgeMediaPath });
        setFlash('Media version purged.');
        await fetchAll(false);
        if (state.mediaHistoryReference) {
          await loadMediaHistory(state.mediaHistoryReference, false);
        }
        navigate('history');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-edit-user]').forEach((button) => {
    button.addEventListener('click', () => {
      const user = state.users.find((item) => item.username === button.dataset.editUser);
      if (!user) return;
      resetUserSecurity();
      setUserForm(
        {
          username: user.username,
          name: user.name || '',
          email: user.email || '',
          role: user.role || '',
          password: '',
          disabled: !!user.disabled,
        },
        { snapshot: true }
      );
      setFlash(`Editing ${user.username}.`);
      navigate('users');
    });
  });

  root.querySelectorAll('[data-edit-document-path]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        const detail = await admin.documents.get(button.dataset.editDocumentPath, {
          include_drafts: 1,
        });
        await loadDocumentIntoEditor(detail);
        setFlash('Document loaded.');
        navigate('editor');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-select-trash]').forEach((checkbox) => {
    checkbox.addEventListener('change', () => {
      if (checkbox.dataset.trashKind === 'document') {
        state.selectedDocumentTrash = toggleSelection(
          state.selectedDocumentTrash,
          checkbox.dataset.selectTrash,
          checkbox.checked
        );
      } else {
        state.selectedMediaTrash = toggleSelection(
          state.selectedMediaTrash,
          checkbox.dataset.selectTrash,
          checkbox.checked
        );
      }
      render();
    });
  });

  root.querySelectorAll('[data-select-document]').forEach((checkbox) => {
    checkbox.addEventListener('change', () => {
      state.selectedDocuments = toggleSelection(
        state.selectedDocuments,
        checkbox.dataset.selectDocument,
        checkbox.checked
      );
      render();
    });
  });

  root.querySelectorAll('[data-select-media-library]').forEach((checkbox) => {
    checkbox.addEventListener('change', () => {
      state.selectedMediaLibrary = toggleSelection(
        state.selectedMediaLibrary,
        checkbox.dataset.selectMediaLibrary,
        checkbox.checked
      );
      render();
    });
  });

  document.getElementById('document-select-all-visible')?.addEventListener('click', () => {
    const visible = Array.from(root.querySelectorAll('[data-select-document]')).map(
      (node) => node.dataset.selectDocument
    );
    state.selectedDocuments =
      state.selectedDocuments.length === visible.length ? [] : visible.filter(Boolean);
    render();
  });

  document.getElementById('document-clear-selection')?.addEventListener('click', () => {
    state.selectedDocuments = [];
    render();
  });

  document.getElementById('media-select-all-visible')?.addEventListener('click', () => {
    const visible = Array.from(root.querySelectorAll('[data-select-media-library]')).map(
      (node) => node.dataset.selectMediaLibrary
    );
    state.selectedMediaLibrary =
      state.selectedMediaLibrary.length === visible.length ? [] : visible.filter(Boolean);
    render();
  });

  document.getElementById('media-clear-selection')?.addEventListener('click', () => {
    state.selectedMediaLibrary = [];
    render();
  });

  document.getElementById('document-bulk-apply')?.addEventListener('click', async () => {
    if (!state.selectedDocuments.length) return;
    state.documentBulk = {
      status: document.getElementById('document-bulk-status')?.value || '',
      author: document.getElementById('document-bulk-author')?.value || '',
      lang: document.getElementById('document-bulk-lang')?.value || '',
      tags: document.getElementById('document-bulk-tags')?.value || '',
      categories: document.getElementById('document-bulk-categories')?.value || '',
    };
    if (
      !window.confirm(
        `Apply bulk updates to ${state.selectedDocuments.length} selected document(s)?`
      )
    )
      return;
    try {
      for (const sourcePath of state.selectedDocuments) {
        const detail = await admin.documents.get(sourcePath, { include_drafts: 1 });
        let raw = detail.raw_body || '';
        const parsed = parseDocumentEditor(raw, sourcePath);
        if (state.documentBulk.author) parsed.extraLines = parsed.extraLines.filter((line) => !/^author:\s*/i.test(line));
        if (state.documentBulk.author) parsed.extraLines.push(`author: ${JSON.stringify(state.documentBulk.author)}`);
        if (state.documentBulk.lang) parsed.fields.lang = state.documentBulk.lang;
        if (state.documentBulk.tags) {
          parsed.fields.tags = Array.from(
            new Set([...(parsed.fields.tags || []), ...parseTagInput(state.documentBulk.tags)])
          );
        }
        if (state.documentBulk.categories) {
          parsed.fields.categories = Array.from(
            new Set([
              ...(parsed.fields.categories || []),
              ...parseTagInput(state.documentBulk.categories),
            ])
          );
        }
        raw = buildDocumentRaw(parsed.fields, parsed.body, parsed.extraLines, parsed.fields.lang || 'en');
        await admin.documents.save({ source_path: sourcePath, raw, version_comment: 'Bulk editorial update' });
        if (state.documentBulk.status) {
          await admin.documents.setStatus({ source_path: sourcePath, status: state.documentBulk.status });
        }
      }
      state.selectedDocuments = [];
      setFlash('Bulk document updates applied.');
      await fetchAll(false);
      navigate('documents');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('media-bulk-apply')?.addEventListener('click', async () => {
    if (!state.selectedMediaLibrary.length) return;
    state.mediaBulkTags = document.getElementById('media-bulk-tags')?.value || '';
    if (
      !window.confirm(`Append tags to ${state.selectedMediaLibrary.length} selected media item(s)?`)
    )
      return;
    try {
      for (const reference of state.selectedMediaLibrary) {
        const detail = await admin.media.getDetail(reference);
        await admin.media.updateMetadata({
          reference,
          version_comment: 'Bulk media tag update',
          metadata: {
            ...detail.metadata,
            tags: Array.from(
              new Set([...(detail.metadata?.tags || []), ...parseTagInput(state.mediaBulkTags)])
            ),
          },
        });
      }
      state.selectedMediaLibrary = [];
      setFlash('Bulk media tags applied.');
      await fetchAll(false);
      navigate('media');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('document-trash-select-all')?.addEventListener('click', () => {
    state.selectedDocumentTrash =
      state.selectedDocumentTrash.length === state.documentTrash.length
        ? []
        : state.documentTrash.map((entry) => entry.path);
    render();
  });

  document.getElementById('media-trash-select-all')?.addEventListener('click', () => {
    state.selectedMediaTrash =
      state.selectedMediaTrash.length === state.mediaTrash.length
        ? []
        : state.mediaTrash.map((entry) => entry.path);
    render();
  });

  document
    .getElementById('document-trash-restore-selected')
    ?.addEventListener('click', async () => {
      if (
        !state.selectedDocumentTrash.length ||
        !window.confirm(`Restore ${state.selectedDocumentTrash.length} selected document(s)?`)
      )
        return;
      try {
        let lastRestoredPath = '';
        for (const path of state.selectedDocumentTrash) {
          const restored = await admin.documents.restore({ path });
          lastRestoredPath = restored.restored_path || restored.path;
        }
        state.selectedDocumentTrash = [];
        await fetchAll(false);
        if (lastRestoredPath) {
          await loadDocumentHistory(lastRestoredPath, false);
          const detail = await admin.documents.get(lastRestoredPath, { include_drafts: 1 });
          await loadDocumentIntoEditor(detail);
        }
        setFlash('Selected documents restored.');
        navigate(lastRestoredPath ? 'editor' : 'trash');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });

  document.getElementById('document-trash-purge-selected')?.addEventListener('click', async () => {
    if (
      !state.selectedDocumentTrash.length ||
      !window.confirm(
        `Permanently purge ${state.selectedDocumentTrash.length} selected document(s)?`
      )
    )
      return;
    try {
      for (const path of state.selectedDocumentTrash) {
        await admin.documents.purge({ path });
      }
      state.selectedDocumentTrash = [];
      await fetchAll(false);
      setFlash('Selected documents purged.');
      navigate('trash');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('media-trash-restore-selected')?.addEventListener('click', async () => {
    if (
      !state.selectedMediaTrash.length ||
      !window.confirm(`Restore ${state.selectedMediaTrash.length} selected media item(s)?`)
    )
      return;
    try {
      for (const path of state.selectedMediaTrash) {
        await admin.media.restore({ path });
      }
      state.selectedMediaTrash = [];
      await fetchAll(false);
      setFlash('Selected media restored.');
      navigate('trash');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('media-trash-purge-selected')?.addEventListener('click', async () => {
    if (
      !state.selectedMediaTrash.length ||
      !window.confirm(
        `Permanently purge ${state.selectedMediaTrash.length} selected media item(s)?`
      )
    )
      return;
    try {
      for (const path of state.selectedMediaTrash) {
        await admin.media.purge({ path });
      }
      state.selectedMediaTrash = [];
      await fetchAll(false);
      setFlash('Selected media purged.');
      navigate('trash');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('user-save-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    try {
      await admin.users.save({
        username: document.getElementById('user-username').value,
        name: document.getElementById('user-name').value,
        email: document.getElementById('user-email').value,
        role: document.getElementById('user-role').value,
        password: document.getElementById('user-password').value,
        disabled: document.getElementById('user-disabled').checked,
      });
      resetUserForm({ snapshot: true });
      resetUserSecurity();
      setFlash('User saved.');
      await fetchAll(false);
      navigate('users');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('user-reset-button')?.addEventListener('click', () => {
    resetUserForm({ snapshot: true });
    resetUserSecurity();
    setFlash('User form reset.');
    render();
  });

  root.querySelectorAll('[data-delete-user]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        await admin.users.delete({ username: button.dataset.deleteUser });
        resetUserForm({ snapshot: true });
        resetUserSecurity();
        setFlash('User deleted.');
        await fetchAll(false);
        navigate('users');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  document.getElementById('user-revoke-sessions')?.addEventListener('click', async () => {
    const selectedUser = selectedUserRecord();
    if (!selectedUser) return;
    try {
      const resp = await admin.session.revoke({ username: selectedUser.username });
      setFlash(`Revoked ${resp.revoked || 0} session(s) for ${selectedUser.username}.`);
      await fetchAll(false);
      navigate('users');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('user-revoke-all-sessions')?.addEventListener('click', async () => {
    if (!window.confirm('Revoke all active admin sessions?')) return;
    try {
      const resp = await admin.session.revoke({ all: true });
      setFlash(`Revoked ${resp.revoked || 0} session(s).`);
      await fetchAll(false);
      navigate('users');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('user-reset-start')?.addEventListener('click', async () => {
    const selectedUser = selectedUserRecord();
    if (!selectedUser) return;
    try {
      state.userSecurity.resetStart = await admin.session.startPasswordReset({
        username: selectedUser.username,
      });
      setFlash(`Password reset token issued for ${selectedUser.username}.`);
      render();
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('user-reset-complete-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const selectedUser = selectedUserRecord();
    if (!selectedUser) return;
    try {
      await admin.session.completePasswordReset({
        username: selectedUser.username,
        reset_token: document.getElementById('user-reset-token').value,
        new_password: document.getElementById('user-reset-password').value,
        totp_code: document.getElementById('user-reset-totp').value,
      });
      state.userSecurity.resetStart = null;
      setFlash(`Password reset completed for ${selectedUser.username}.`);
      render();
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('user-totp-setup')?.addEventListener('click', async () => {
    const selectedUser = selectedUserRecord();
    if (!selectedUser) return;
    try {
      state.userSecurity.totpSetup = await admin.session.setupTOTP({
        username: selectedUser.username,
      });
      setFlash(`TOTP setup generated for ${selectedUser.username}.`);
      render();
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('user-totp-enable-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    const selectedUser = selectedUserRecord();
    if (!selectedUser) return;
    try {
      await admin.session.enableTOTP({
        username: selectedUser.username,
        code: document.getElementById('user-totp-enable-code').value,
      });
      state.userSecurity.totpSetup = null;
      setFlash(`TOTP enabled for ${selectedUser.username}.`);
      await fetchAll(false);
      navigate('users');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('user-totp-cancel')?.addEventListener('click', () => {
    state.userSecurity.totpSetup = null;
    render();
  });

  document.getElementById('user-totp-disable')?.addEventListener('click', async () => {
    const selectedUser = selectedUserRecord();
    if (!selectedUser) return;
    if (!window.confirm(`Disable TOTP for ${selectedUser.username}?`)) return;
    try {
      await admin.session.disableTOTP({ username: selectedUser.username });
      state.userSecurity.totpSetup = null;
      setFlash(`TOTP disabled for ${selectedUser.username}.`);
      await fetchAll(false);
      navigate('users');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('config-save-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    try {
      await admin.settings.saveConfig({ raw: document.getElementById('config-raw').value });
      setFlash('Configuration saved.');
      snapshotValue('config', document.getElementById('config-raw').value);
      await fetchAll(false);
      state.settingsTab = 'config';
      navigate('settings');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('custom-css-save-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    try {
      await admin.settings.saveCustomCSS({ raw: document.getElementById('custom-css-raw').value });
      setFlash('Custom CSS saved.');
      snapshotValue('customCss', document.getElementById('custom-css-raw').value);
      await fetchAll(false);
      state.settingsTab = 'custom-css';
      navigate('settings');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  root.querySelectorAll('[data-enable-plugin]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        const name = button.dataset.enablePlugin;
        try {
          await admin.plugins.enable({ name });
        } catch (error) {
          if (!isRiskAckError(error)) {
            throw error;
          }
          const plugin = pluginRecordByName(name);
          if (!confirmPluginRisk('Enable', name, plugin)) {
            return;
          }
          await admin.plugins.enable({
            name,
            approve_risk: true,
            acknowledge_mismatches: true,
          });
        }
        setFlash('Plugin enabled.');
        await fetchAll(false);
        navigate('plugins');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-disable-plugin]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        await admin.plugins.disable(button.dataset.disablePlugin);
        setFlash('Plugin disabled.');
        await fetchAll(false);
        navigate('plugins');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  document.getElementById('plugin-install-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    try {
      const input = {
        url: document.getElementById('plugin-install-url').value,
        name: document.getElementById('plugin-install-name').value,
      };
      let record;
      try {
        record = await admin.plugins.install(input);
      } catch (error) {
        if (!isRiskAckError(error)) {
          throw error;
        }
        if (
          !window.confirm(
            [
              `Install plugin from "${input.url}"?`,
              '',
              'Foundry detected a plugin that requires explicit risk approval or mismatch acknowledgment.',
              'Continuing will download the plugin and accept the reported risk if validation finds one.',
              '',
              'Continue?',
            ].join('\n')
          )
        ) {
          return;
        }
        record = await admin.plugins.install({
          ...input,
          approve_risk: true,
          acknowledge_mismatches: true,
        });
      }
      setFlash(`Plugin ${record.name || 'installed'} installed.`);
      await fetchAll(false);
      navigate('plugins');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  root.querySelectorAll('[data-update-plugin]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        const name = button.dataset.updatePlugin;
        try {
          await admin.plugins.update({ name });
        } catch (error) {
          if (!isRiskAckError(error)) {
            throw error;
          }
          const plugin = pluginRecordByName(name);
          if (!confirmPluginRisk('Update', name, plugin)) {
            return;
          }
          await admin.plugins.update({
            name,
            approve_risk: true,
            acknowledge_mismatches: true,
          });
        }
        setFlash('Plugin updated.');
        await fetchAll(false);
        navigate('plugins');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-validate-plugin]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        const record = await admin.plugins.validate(button.dataset.validatePlugin);
        setFlash(
          `Plugin ${record.valid === false || record.health === 'invalid' || record.health === 'degraded' ? 'validation found issues' : 'validated successfully'}.`
        );
        await fetchAll(false);
        navigate('plugins');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-rollback-plugin]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        await admin.plugins.rollback({ name: button.dataset.rollbackPlugin });
        setFlash('Plugin rolled back.');
        await fetchAll(false);
        navigate('plugins');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  root.querySelectorAll('[data-switch-theme]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        if ((button.dataset.themeKind || 'frontend') === 'admin') {
          await admin.themes.switchAdmin(button.dataset.switchTheme);
        } else {
          await admin.themes.switchFrontend(button.dataset.switchTheme);
        }
        setFlash('Theme switched.');
        await fetchAll(false);
        navigate('themes');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  document.getElementById('theme-install-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    try {
      const record = await admin.themes.install({
        url: document.getElementById('theme-install-url').value,
        name: document.getElementById('theme-install-name').value,
        kind: document.getElementById('theme-install-kind').value || 'frontend',
      });
      setFlash(
        `${record.kind === 'admin' ? 'Admin theme' : 'Theme'} ${record.name || 'installed'} installed.`
      );
      await fetchAll(false);
      navigate('themes');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  root.querySelectorAll('[data-validate-theme]').forEach((button) => {
    button.addEventListener('click', async () => {
      try {
        const record = await admin.themes.validate({
          name: button.dataset.validateTheme,
          kind: button.dataset.themeKind || 'frontend',
        });
        setFlash(`Theme ${record.valid ? 'validated successfully' : 'validation found issues'}.`);
        await fetchAll(false);
        navigate('themes');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  document.getElementById('backup-create')?.addEventListener('click', async () => {
    try {
      const record = await admin.backups.create({});
      setFlash(`Backup ${record.name || 'created'} created.`);
      await fetchAll(false);
      navigate('operations');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  root.querySelectorAll('[data-restore-backup]').forEach((button) => {
    button.addEventListener('click', async () => {
      if (!window.confirm(`Restore backup ${button.dataset.restoreBackup}? The current content tree will be snapshotted first.`)) {
        return;
      }
      try {
        await admin.backups.restore({ name: button.dataset.restoreBackup });
        setFlash('Backup restored.');
        await fetchAll(false);
        navigate('operations');
      } catch (error) {
        state.error = error.message || String(error);
        render();
      }
    });
  });

  document.getElementById('backup-git-create')?.addEventListener('click', async () => {
    try {
      const record = await admin.backups.createGit({});
      setFlash(`Git snapshot ${record.revision?.slice(0, 12) || 'created'} created.`);
      await fetchAll(false);
      navigate('operations');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('backup-git-push')?.addEventListener('click', async () => {
    try {
      const record = await admin.backups.createGit({ push: true });
      setFlash(
        `Git snapshot ${record.revision?.slice(0, 12) || 'created'} ${record.pushed ? 'pushed' : 'created'}.`
      );
      await fetchAll(false);
      navigate('operations');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('operations-backup-git-form')?.addEventListener('submit', async (event) => {
    event.preventDefault();
    try {
      const next = structuredClone(state.settingsForm || {});
      next.Backup = next.Backup || {};
      next.Backup.GitRemoteURL = document.getElementById('operations-git-remote-url')?.value || '';
      next.Backup.GitBranch = document.getElementById('operations-git-branch')?.value || 'main';
      next.Backup.GitPushOnChange = !!document.getElementById('operations-git-push-on-change')?.checked;
      const saved =
        typeof admin.settings?.saveForm === 'function'
          ? await admin.settings.saveForm({ value: next })
          : await admin.raw.post('/api/settings/form/save', { value: next });
      state.settingsForm = saved?.value || next;
      setFlash('Git backup settings saved.');
      await fetchAll(false);
      navigate('operations');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('update-refresh')?.addEventListener('click', async () => {
    try {
      state.updateInfo = await admin.updates.get();
      setFlash('Update status refreshed.');
      render();
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('operations-refresh')?.addEventListener('click', async () => {
    try {
      state.operationsStatus = await admin.operations.get();
      state.gitBackups = await admin.backups.listGit();
      state.operationsLog = await admin.operations.logs();
      setFlash('Operations status refreshed.');
      render();
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('operations-clear-cache')?.addEventListener('click', async () => {
    try {
      await admin.operations.clearCache();
      setFlash('Runtime cache cleared.');
      await fetchAll(false);
      navigate('operations');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('operations-rebuild')?.addEventListener('click', async () => {
    if (!window.confirm('Run a full Foundry build now?')) {
      return;
    }
    try {
      await admin.operations.rebuild();
      setFlash('Build completed.');
      await fetchAll(false);
      navigate('operations');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('operations-logs-refresh')?.addEventListener('click', async () => {
    try {
      state.operationsLog = await admin.operations.logs();
      setFlash('Logs refreshed.');
      render();
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('operations-validate')?.addEventListener('click', async () => {
    try {
      state.siteValidation = await admin.operations.validate();
      setFlash(`Site validation complete. ${state.siteValidation?.message_count || 0} finding(s).`);
      render();
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('update-apply')?.addEventListener('click', async () => {
    if (!window.confirm('Apply the latest Foundry release and restart the standalone runtime?')) {
      return;
    }
    try {
      state.error = '';
      state.flash = 'Scheduling Foundry update...';
      render();
      const resp = await admin.updates.apply();
      state.updateInfo = resp || state.updateInfo;
      setFlash(
        `Update to ${resp?.latest_version || 'the latest release'} scheduled. Refresh logs below to follow progress.`
      );
      try {
        state.operationsLog = await admin.operations.logs();
      } catch (_error) {}
      try {
        state.operationsStatus = await admin.operations.get();
      } catch (_error) {}
      render();
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  [
    'media-title',
    'media-alt',
    'media-caption',
    'media-description',
    'media-credit',
    'media-focal-x',
    'media-focal-y',
    'media-tags',
    'media-version-comment',
  ].forEach((id) => {
    document.getElementById(id)?.addEventListener('input', () => {
      compareSnapshot('media', {
        reference: state.selectedMediaReference,
        metadata: {
          title: document.getElementById('media-title')?.value || '',
          alt: document.getElementById('media-alt')?.value || '',
          caption: document.getElementById('media-caption')?.value || '',
          description: document.getElementById('media-description')?.value || '',
          credit: document.getElementById('media-credit')?.value || '',
          focal_x: document.getElementById('media-focal-x')?.value || '',
          focal_y: document.getElementById('media-focal-y')?.value || '',
          tags: parseTagInput(document.getElementById('media-tags')?.value || ''),
        },
        versionComment: document.getElementById('media-version-comment')?.value || '',
      });
    });
  });

  [
    'user-username',
    'user-name',
    'user-email',
    'user-role',
    'user-password',
    'user-disabled',
  ].forEach((id) => {
    const node = document.getElementById(id);
    node?.addEventListener(node.type === 'checkbox' ? 'change' : 'input', () => {
      compareSnapshot('user', {
        username: document.getElementById('user-username')?.value || '',
        name: document.getElementById('user-name')?.value || '',
        email: document.getElementById('user-email')?.value || '',
        role: document.getElementById('user-role')?.value || '',
        password: document.getElementById('user-password')?.value || '',
        disabled: !!document.getElementById('user-disabled')?.checked,
      });
    });
  });

  document.getElementById('config-raw')?.addEventListener('input', () => {
    compareSnapshot('config', document.getElementById('config-raw').value);
  });

  document.getElementById('custom-css-raw')?.addEventListener('input', () => {
    compareSnapshot('customCss', document.getElementById('custom-css-raw').value);
  });

  document.getElementById('audit-filter-apply')?.addEventListener('click', () => {
    state.auditFilters = {
      actor: document.getElementById('audit-filter-actor')?.value.trim() || '',
      action: document.getElementById('audit-filter-action')?.value.trim() || '',
      outcome: document.getElementById('audit-filter-outcome')?.value || '',
    };
    render();
  });

  document.getElementById('audit-filter-clear')?.addEventListener('click', () => {
    state.auditFilters = { actor: '', action: '', outcome: '' };
    render();
  });
  document.getElementById('debug-refresh-runtime')?.addEventListener('click', async () => {
    try {
      state.runtimeStatus = await admin.raw.get('/api/debug/runtime');
      setFlash('Runtime snapshot refreshed.');
      render();
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('debug-validate-site')?.addEventListener('click', async () => {
    try {
      state.siteValidation = await admin.raw.post('/api/debug/validate', {});
      setFlash(
        `Site validation complete. ${state.siteValidation?.message_count || 0} finding(s).`
      );
      render();
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });

  document.getElementById('overview-validate-site')?.addEventListener('click', async () => {
    try {
      state.siteValidation = await admin.raw.post('/api/debug/validate', {});
      setFlash(
        `Site validation complete. ${state.siteValidation?.message_count || 0} finding(s).`
      );
      navigate('debug');
    } catch (error) {
      state.error = error.message || String(error);
      render();
    }
  });
};
