// createUIStateHelpers centralizes non-network UI state transitions for the
// default admin theme.
//
// Keeping these helpers in one module avoids spreading dirty-state, snapshots,
// pagination, and toast behavior across unrelated view code.
export const createUIStateHelpers = ({ state, render, buildDefaultMarkdown }) => {
  const setUserForm = (value, { snapshot = false } = {}) => {
    state.userForm = {
      username: value?.username || '',
      name: value?.name || '',
      email: value?.email || '',
      role: value?.role || '',
      password: value?.password || '',
      disabled: !!value?.disabled,
    };
    if (snapshot) {
      snapshotValue('user', state.userForm);
    }
  };

  const resetUserForm = (options) => {
    setUserForm(
      { username: '', name: '', email: '', role: '', password: '', disabled: false },
      options
    );
  };

  const resetUserSecurity = () => {
    state.userSecurity = { resetStart: null, totpSetup: null };
  };

  const selectedUserRecord = () => {
    const username = String(state.userForm?.username || '').trim();
    if (!username) return null;
    return state.users.find((item) => item.username === username) || null;
  };

  const resetDocumentEditor = () => {
    const createKind = document.getElementById('document-create-kind')?.value || 'post';
    state.documentEditor = {
      source_path: '',
      raw: buildDefaultMarkdown(createKind),
      version_comment: '',
      lock_token: '',
    };
    state.documentFieldSchema = [];
    state.documentFieldValues = {};
    state.documentContractTitles = [];
    state.documentMeta = {
      status: 'draft',
      author: '',
      last_editor: '',
      created_at: '',
      updated_at: '',
    };
    state.documentLock = null;
    state.documentPreview = null;
  };

  const pushToast = (message, tone = 'info') => {
    if (!String(message || '').trim()) return;
    const id = Date.now() + Math.random();
    state.toasts = [...state.toasts.slice(-3), { id, message: String(message), tone }];
    window.setTimeout(
      () => {
        state.toasts = state.toasts.filter((toast) => toast.id !== id);
        render();
      },
      tone === 'error' ? 6500 : 3500
    );
  };

  const setFlash = (message) => {
    state.flash = message;
    state.error = '';
    pushToast(message, 'success');
  };

  const setError = (message) => {
    state.error = message;
    if (message) {
      pushToast(message, 'error');
    }
  };

  const markDirty = (key, next = true) => {
    state.dirty[key] = next;
  };

  const snapshotValue = (key, value) => {
    state.snapshots[key] = JSON.stringify(value ?? '');
    state.dirty[key] = false;
  };

  const dirtyMessage = () =>
    Object.entries(state.dirty)
      .filter(([, value]) => value)
      .map(([key]) => ({ customCss: 'custom css', customFields: 'shared custom fields', settings: 'settings form' }[key] || key))
      .join(', ');

  const hasUnsavedChanges = () => Object.values(state.dirty).some(Boolean);

  const clearDirtyState = () => {
    for (const key of Object.keys(state.dirty)) {
      state.dirty[key] = false;
    }
  };

  const confirmNavigation = () => {
    if (!hasUnsavedChanges()) return true;
    const confirmed = window.confirm(
      `You have unsaved changes in: ${dirtyMessage()}. Leave this view?`
    );
    if (confirmed) {
      clearDirtyState();
    }
    return confirmed;
  };

  const compareSnapshot = (key, value) => {
    state.dirty[key] = state.snapshots[key] !== JSON.stringify(value ?? '');
  };

  const updateTablePage = (name, page) => {
    const table = state.tables[name];
    if (!table) return;
    table.page = Math.max(1, page);
  };

  const sortItems = (items, tableName, valueFor) => {
    const table = state.tables[tableName];
    if (!table) return items;
    return [...items].sort((left, right) => {
      const a = String(valueFor(left, table.sort) ?? '').toLowerCase();
      const b = String(valueFor(right, table.sort) ?? '').toLowerCase();
      if (a === b) return 0;
      const cmp = a < b ? -1 : 1;
      return table.dir === 'asc' ? cmp : -cmp;
    });
  };

  const paginateItems = (items, tableName) => {
    const table = state.tables[tableName];
    if (!table) return { items, totalPages: 1, page: 1 };
    const totalPages = Math.max(1, Math.ceil(items.length / table.pageSize));
    table.page = Math.min(table.page, totalPages);
    const start = (table.page - 1) * table.pageSize;
    return { items: items.slice(start, start + table.pageSize), totalPages, page: table.page };
  };

  const toggleSelection = (items, value, checked) =>
    checked ? Array.from(new Set([...items, value])) : items.filter((item) => item !== value);

  const clearLoadErrors = () => {
    state.loadErrors = [];
  };

  return {
    setUserForm,
    resetUserForm,
    resetUserSecurity,
    selectedUserRecord,
    resetDocumentEditor,
    setFlash,
    setError,
    pushToast,
    markDirty,
    snapshotValue,
    dirtyMessage,
    hasUnsavedChanges,
    clearDirtyState,
    confirmNavigation,
    compareSnapshot,
    updateTablePage,
    sortItems,
    paginateItems,
    toggleSelection,
    clearLoadErrors,
  };
};
