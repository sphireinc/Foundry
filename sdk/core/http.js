import { normalizeFoundryError } from './errors.js';

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS']);

const joinURL = (baseURL, path) => {
  const base = String(baseURL || '').replace(/\/+$/, '');
  const next = String(path || '').trim();
  if (!next) return base || '';
  if (/^https?:\/\//i.test(next)) return next;
  if (!base) return next.startsWith('/') ? next : `/${next}`;
  return `${base}${next.startsWith('/') ? next : `/${next}`}`;
};

export const buildQueryString = (query = {}) => {
  const params = new URLSearchParams();
  Object.entries(query || {}).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return;
    if (Array.isArray(value)) {
      value.forEach((entry) => {
        if (entry !== undefined && entry !== null && entry !== '') {
          params.append(key, String(entry));
        }
      });
      return;
    }
    params.set(key, String(value));
  });
  const encoded = params.toString();
  return encoded ? `?${encoded}` : '';
};

const parseBody = async (response) => {
  const contentType = response.headers.get('content-type') || '';
  if (contentType.includes('application/json')) {
    return response.json().catch(() => ({}));
  }
  const text = await response.text().catch(() => '');
  return text ? { message: text } : {};
};

export const createHttpClient = ({
  baseURL = '',
  fetchImpl = globalThis.fetch?.bind(globalThis),
  headers = {},
  credentials = 'same-origin',
  prepareRequest,
} = {}) => {
  if (typeof fetchImpl !== 'function') {
    throw new Error('Foundry SDK requires a fetch implementation');
  }

  const request = async (path, options = {}) => {
    const method = String(options.method || 'GET').toUpperCase();
    const query = buildQueryString(options.query);
    const url = joinURL(baseURL, `${path}${query}`);
    const requestHeaders = { ...(headers || {}), ...(options.headers || {}) };
    const requestOptions = {
      method,
      credentials: options.credentials || credentials,
      headers: requestHeaders,
    };

    if (options.body instanceof FormData) {
      requestOptions.body = options.body;
    } else if (options.body !== undefined && options.body !== null) {
      requestHeaders['Content-Type'] = requestHeaders['Content-Type'] || 'application/json';
      requestOptions.body =
        typeof options.body === 'string' ? options.body : JSON.stringify(options.body);
    }

    if (typeof prepareRequest === 'function') {
      await prepareRequest({
        method,
        path,
        url,
        safe: SAFE_METHODS.has(method),
        headers: requestHeaders,
        options: requestOptions,
      });
    }

    let response;
    try {
      response = await fetchImpl(url, requestOptions);
    } catch (cause) {
      throw normalizeFoundryError({
        fallbackMessage: `Network error while requesting ${url}`,
        cause,
      });
    }

    const payload = await parseBody(response);
    if (!response.ok) {
      throw normalizeFoundryError({
        response,
        payload,
        fallbackMessage: `Request failed for ${url}`,
      });
    }
    return payload;
  };

  return {
    request,
    get: (path, options = {}) => request(path, { ...options, method: 'GET' }),
    post: (path, body, options = {}) => request(path, { ...options, method: 'POST', body }),
    put: (path, body, options = {}) => request(path, { ...options, method: 'PUT', body }),
    delete: (path, body, options = {}) => request(path, { ...options, method: 'DELETE', body }),
  };
};
