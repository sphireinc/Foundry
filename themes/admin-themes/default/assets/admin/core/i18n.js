const LANGUAGE_STORAGE_KEY = 'foundry.admin.ui-language';
const LANGUAGE_NAMES = { en: 'English', es: 'Español' };

let localeCatalog = {};
let activeLocale = 'en';
const patternCache = new Map();

const normalizeLocale = (value) =>
  String(value || '')
    .trim()
    .replaceAll('_', '-')
    .toLowerCase();

const owns = (object, key) => Object.prototype.hasOwnProperty.call(object, key);

const supportedLocale = (value) => {
  const normalized = normalizeLocale(value);
  const base = normalized.split('-')[0];
  return owns(localeCatalog, normalized) || owns(localeCatalog, base) || base === 'en' ? base : '';
};

const getInitialLocale = () => {
  try {
    const saved = supportedLocale(window.localStorage.getItem(LANGUAGE_STORAGE_KEY));
    if (saved) return saved;
  } catch (_error) {
    // Storage may be disabled; browser preferences still provide a default.
  }

  const preferences = Array.isArray(window.navigator.languages)
    ? window.navigator.languages
    : [window.navigator.language];
  for (const preference of preferences) {
    const supported = supportedLocale(preference);
    if (supported) return supported;
  }
  return 'en';
};

export const initializeI18n = (root) => {
  try {
    localeCatalog = JSON.parse(root?.dataset?.i18nCatalog || '{}');
    if (!localeCatalog || typeof localeCatalog !== 'object' || Array.isArray(localeCatalog)) {
      localeCatalog = {};
    }
  } catch (_error) {
    localeCatalog = {};
  }
  activeLocale = getInitialLocale();
  document.documentElement.lang = activeLocale;
  return activeLocale;
};

export const getLocale = () => activeLocale;

export const setLocale = (value) => {
  const supported = supportedLocale(value);
  if (!supported) return activeLocale;
  activeLocale = supported;
  document.documentElement.lang = activeLocale;
  try {
    window.localStorage.setItem(LANGUAGE_STORAGE_KEY, activeLocale);
  } catch (_error) {
    // The active page still changes language when storage is unavailable.
  }
  return activeLocale;
};

export const supportedLanguages = () => {
  const codes = new Set([
    'en',
    ...Object.keys(localeCatalog)
      .map((code) => supportedLocale(code))
      .filter(Boolean),
  ]);
  return [...codes].sort().map((code) => ({ code, name: LANGUAGE_NAMES[code] || code }));
};

// _t is intentionally keyed by the English source phrase. Missing phrases and
// locales fall back to the source copy so partial catalogues never blank UI.
export const _t = (key, values = {}) => {
  const locale = supportedLocale(activeLocale) || 'en';
  const base = locale.split('-')[0];
  const messages =
    (owns(localeCatalog, locale) && localeCatalog[locale]) ||
    (owns(localeCatalog, base) && localeCatalog[base]) ||
    {};
  let message = owns(messages, key) && typeof messages[key] === 'string' ? messages[key] : '';
  if (!message) {
    let patterns = patternCache.get(base);
    if (!patterns) {
      patterns = Object.entries(messages)
        .filter(([source]) => source.includes('{'))
        .map(([source, translation]) => {
          const names = [...source.matchAll(/\{(\w+)\}/g)].map((match) => match[1]);
          const expressionSource = source
            .split(/(\{\w+\})/g)
            .map((part) =>
              /^\{\w+\}$/.test(part) ? '([\\s\\S]*?)' : part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
            )
            .join('');
          const expression = new RegExp(`^${expressionSource}$`);
          return { source, translation, names, expression };
        });
      patternCache.set(base, patterns);
    }
    for (const pattern of patterns) {
      const match = String(key).match(pattern.expression);
      if (!match) continue;
      message = pattern.translation;
      pattern.names.forEach((name, index) => {
        message = message.replaceAll(`{${name}}`, () => match[index + 1]);
      });
      break;
    }
  }
  if (!message) message = key;
  for (const [name, value] of Object.entries(values)) {
    message = message.replaceAll(`{${name}}`, () => String(value));
  }
  return message;
};
