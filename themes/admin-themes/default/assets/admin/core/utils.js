export const escapeHTML = (value) =>
  String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');

export const formatDate = (value) => value.toISOString().slice(0, 10);

export const formatDateTime = (value) => {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
};

export const lifecycleLabel = (value) => {
  switch (value) {
    case 'version':
      return 'Version';
    case 'trash':
      return 'Trash';
    default:
      return 'Current';
  }
};

export const slugify = (value) =>
  String(value || '')
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');

export const parseBool = (value) => {
  const normalized = String(value || '')
    .trim()
    .toLowerCase();
  if (normalized === 'true') return true;
  if (normalized === 'false') return false;
  return false;
};

export const stripQuotes = (value) => {
  const trimmed = String(value || '').trim();
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"')) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
};

export const parseInlineList = (value) => {
  const trimmed = String(value || '').trim();
  if (!trimmed || trimmed === '[]') return [];
  const inner = trimmed.startsWith('[') && trimmed.endsWith(']') ? trimmed.slice(1, -1) : trimmed;
  return inner
    .split(',')
    .map((item) => stripQuotes(item))
    .map((item) => item.trim())
    .filter(Boolean);
};

export const parseTagInput = (value) =>
  String(value || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);

export const clone = (value) => JSON.parse(JSON.stringify(value ?? null));

export const getValueAtPath = (value, path) => {
  let current = value;
  for (const segment of path) {
    if (current == null) return undefined;
    current = current[segment];
  }
  return current;
};

export const setValueAtPath = (value, path, nextValue) => {
  if (!path.length) return nextValue;
  const [head, ...tail] = path;
  const container = Array.isArray(value) ? [...value] : { ...(value || {}) };
  container[head] = setValueAtPath(container[head], tail, nextValue);
  return container;
};

export const removeValueAtPath = (value, path) => {
  if (!path.length) return value;
  const [head, ...tail] = path;
  if (!tail.length) {
    if (Array.isArray(value)) {
      const copy = [...value];
      copy.splice(Number(head), 1);
      return copy;
    }
    const copy = { ...(value || {}) };
    delete copy[head];
    return copy;
  }
  const container = Array.isArray(value) ? [...value] : { ...(value || {}) };
  container[head] = removeValueAtPath(container[head], tail);
  return container;
};
