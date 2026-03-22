import {
  clone,
  formatDate,
  parseBool,
  parseInlineList,
  parseTagInput,
  removeValueAtPath,
  setValueAtPath,
  stripQuotes,
} from '../core/utils.js';

export const buildDefaultMarkdown = (kind = 'post') => {
  const today = formatDate(new Date());
  if (kind === 'page') {
    return [
      '---',
      'title: ',
      'slug: ',
      'layout: page',
      'draft: true',
      'workflow: draft',
      'summary: ""',
      'tags: []',
      'categories: []',
      '---',
      '',
      '# Title',
      '',
    ].join('\n');
  }
  return [
    '---',
    'title: ',
    'slug: ',
    'layout: post',
    'draft: true',
    'summary: ""',
    `date: ${today}`,
    'workflow: draft',
    'tags: []',
    'categories: []',
    '---',
    '',
    '# Title',
    '',
  ].join('\n');
};

export const inferLangFromSourcePath = (sourcePath, defaultLang) => {
  const normalized = String(sourcePath || '')
    .replaceAll('\\', '/')
    .replace(/^content\//, '');
  const parts = normalized.split('/').filter(Boolean);
  if (parts.length >= 3 && (parts[0] === 'pages' || parts[0] === 'posts')) {
    return parts[1];
  }
  return defaultLang;
};

export const splitDocumentRaw = (raw) => {
  const normalized = String(raw || '').replaceAll('\r\n', '\n');
  const lines = normalized.split('\n');
  if (lines[0] !== '---') {
    return { hasFrontmatter: false, frontmatter: '', body: normalized };
  }
  for (let index = 1; index < lines.length; index += 1) {
    if (lines[index] === '---') {
      return {
        hasFrontmatter: true,
        frontmatter: lines.slice(1, index).join('\n'),
        body: lines.slice(index + 1).join('\n'),
      };
    }
  }
  return { hasFrontmatter: false, frontmatter: '', body: normalized };
};

export const parseDocumentEditor = (raw, sourcePath = '', defaultLang = 'en') => {
  const split = splitDocumentRaw(raw);
  const fields = {
    title: '',
    slug: '',
    layout: sourcePath.includes('/pages/') ? 'page' : 'post',
    date: '',
    summary: '',
    tags: [],
    categories: [],
    draft: true,
    archived: false,
    lang: inferLangFromSourcePath(sourcePath, defaultLang),
    workflow: 'draft',
    scheduled_publish_at: '',
    scheduled_unpublish_at: '',
    editorial_note: '',
  };
  const extraLines = [];
  const lines = split.frontmatter ? split.frontmatter.split('\n') : [];

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    const match = line.match(/^([A-Za-z0-9_-]+):\s*(.*)$/);
    if (!match) {
      extraLines.push(line);
      continue;
    }

    const key = match[1].toLowerCase();
    const value = match[2];
    if ((key === 'tags' || key === 'categories') && value.trim() === '') {
      const items = [];
      let next = index + 1;
      while (next < lines.length && /^\s*-\s+/.test(lines[next])) {
        items.push(stripQuotes(lines[next].replace(/^\s*-\s+/, '')));
        next += 1;
      }
      if (next !== index + 1) {
        fields[key] = items;
        index = next - 1;
        continue;
      }
    }

    switch (key) {
      case 'title':
      case 'slug':
      case 'layout':
      case 'date':
      case 'summary':
        fields[key] = stripQuotes(value);
        break;
      case 'draft':
      case 'archived':
        fields[key] = parseBool(value);
        break;
      case 'tags':
      case 'categories':
        fields[key] = parseInlineList(value);
        break;
      case 'lang':
      case 'language':
        fields.lang = stripQuotes(value) || inferLangFromSourcePath(sourcePath, defaultLang);
        break;
      case 'workflow':
        fields.workflow = stripQuotes(value) || 'draft';
        break;
      case 'scheduled_publish_at':
      case 'scheduled_unpublish_at':
      case 'editorial_note':
        fields[key] = stripQuotes(value);
        break;
      default:
        extraLines.push(line);
    }
  }

  if (fields.archived) {
    fields.workflow = 'archived';
  } else if (fields.workflow === 'published' && fields.draft) {
    fields.workflow = 'draft';
  }

  return {
    ...split,
    fields,
    extraLines,
  };
};

export const quoteYAML = (value) => {
  const stringValue = String(value ?? '');
  if (stringValue === '') return '""';
  if (/^[A-Za-z0-9._/-]+$/.test(stringValue)) return stringValue;
  return JSON.stringify(stringValue);
};

export const renderYAMLList = (items) => `[${items.map((item) => quoteYAML(item)).join(', ')}]`;

export const buildDocumentRaw = (fields, body, extraLines = [], defaultLang = 'en') => {
  const normalized = {
    title: String(fields.title || '').trim(),
    slug: String(fields.slug || '').trim(),
    layout: String(fields.layout || '').trim() || 'post',
    date: String(fields.date || '').trim(),
    summary: String(fields.summary || ''),
    tags: Array.isArray(fields.tags) ? fields.tags.filter(Boolean) : [],
    categories: Array.isArray(fields.categories) ? fields.categories.filter(Boolean) : [],
    draft: !!fields.draft,
    archived: !!fields.archived,
    lang: String(fields.lang || '').trim(),
    workflow: String(fields.workflow || '').trim() || 'draft',
    scheduled_publish_at: String(fields.scheduled_publish_at || '').trim(),
    scheduled_unpublish_at: String(fields.scheduled_unpublish_at || '').trim(),
    editorial_note: String(fields.editorial_note || ''),
  };
  const lines = [
    '---',
    `title: ${quoteYAML(normalized.title)}`,
    `slug: ${quoteYAML(normalized.slug)}`,
    `layout: ${quoteYAML(normalized.layout)}`,
    `draft: ${normalized.draft ? 'true' : 'false'}`,
    `summary: ${quoteYAML(normalized.summary)}`,
  ];
  if (normalized.date) lines.push(`date: ${quoteYAML(normalized.date)}`);
  lines.push(`tags: ${renderYAMLList(normalized.tags)}`);
  lines.push(`categories: ${renderYAMLList(normalized.categories)}`);
  lines.push(`workflow: ${quoteYAML(normalized.workflow)}`);
  if (normalized.scheduled_publish_at)
    lines.push(`scheduled_publish_at: ${quoteYAML(normalized.scheduled_publish_at)}`);
  if (normalized.scheduled_unpublish_at)
    lines.push(`scheduled_unpublish_at: ${quoteYAML(normalized.scheduled_unpublish_at)}`);
  if (normalized.editorial_note)
    lines.push(`editorial_note: ${quoteYAML(normalized.editorial_note)}`);
  if (normalized.archived) lines.push('archived: true');
  if (normalized.lang && normalized.lang !== defaultLang)
    lines.push(`lang: ${quoteYAML(normalized.lang)}`);
  extraLines.filter((line) => String(line).trim() !== '').forEach((line) => lines.push(line));
  lines.push('---', '', String(body || '').replaceAll('\r\n', '\n'));
  return lines.join('\n');
};

export const defaultValueForSchema = (schema) => {
  if (!schema) return '';
  if (schema.default !== undefined) return clone(schema.default);
  switch (schema.type) {
    case 'bool':
      return false;
    case 'number':
      return '';
    case 'select':
      return schema.enum?.[0] || '';
    case 'object': {
      const out = {};
      (schema.fields || []).forEach((field) => {
        out[field.name] = defaultValueForSchema(field);
      });
      return out;
    }
    case 'repeater':
      return [];
    default:
      return '';
  }
};

export const updateNestedFieldValue = (state, path, nextValue) => {
  state.documentFieldValues = setValueAtPath(state.documentFieldValues, path, nextValue);
};

export const removeNestedFieldValue = (state, path) => {
  state.documentFieldValues = removeValueAtPath(state.documentFieldValues, path);
};

export { parseTagInput };
