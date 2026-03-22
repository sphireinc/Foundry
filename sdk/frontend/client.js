import { createHttpClient } from '../core/http.js';
import { createFrontendCapabilitiesAPI } from './capabilities.js';
import { createFrontendCollectionsAPI } from './collections.js';
import { createFrontendContentAPI } from './content.js';
import { createFrontendMediaAPI } from './media.js';
import { createFrontendNavigationAPI } from './navigation.js';
import { createFrontendPreviewAPI } from './preview.js';
import { createFrontendRoutesAPI } from './routes.js';
import { createFrontendSearchAPI } from './search.js';
import { createFrontendSiteAPI } from './site.js';

export const createFrontendClient = ({
  baseURL = '',
  apiBase = '/__foundry/api',
  staticBase = '/__foundry',
  mode = 'auto',
  fetch: fetchImpl,
} = {}) => {
  const resolvedMode = mode === 'auto' ? 'api' : mode;
  const http = createHttpClient({
    baseURL: `${String(baseURL || '').replace(/\/+$/, '')}${apiBase}`,
    fetchImpl,
  });
  const staticRoot = `${String(baseURL || '').replace(/\/+$/, '')}${staticBase}`;
  const transport = {
    http,
    mode: resolvedMode,
    normalizePath(path) {
      const value = String(path || '/').trim();
      if (!value) return '/';
      return value.startsWith('/') ? value : `/${value}`;
    },
    currentPath() {
      if (typeof window === 'undefined' || !window.location) return '/';
      return this.normalizePath(window.location.pathname);
    },
    async loadStaticJSON(path) {
      const url = `${staticRoot}${path}`;
      const response = await (fetchImpl || globalThis.fetch).call(globalThis, url, {
        credentials: 'same-origin',
      });
      if (!response.ok) {
        throw new Error(`failed to load static Foundry data from ${url}`);
      }
      return response.json();
    },
    async getStaticOrAPI(apiPath, staticPath) {
      if (this.mode === 'static') {
        return this.loadStaticJSON(staticPath);
      }
      try {
        return await http.get(apiPath);
      } catch (error) {
        if (mode === 'auto') {
          this.mode = 'static';
          return this.loadStaticJSON(staticPath);
        }
        throw error;
      }
    },
  };

  const capabilities = createFrontendCapabilitiesAPI(transport);
  const routes = createFrontendRoutesAPI(transport);
  return {
    capabilities,
    site: createFrontendSiteAPI(transport),
    navigation: createFrontendNavigationAPI(transport),
    routes,
    content: createFrontendContentAPI(transport, routes),
    collections: createFrontendCollectionsAPI(transport),
    search: createFrontendSearchAPI(transport),
    media: createFrontendMediaAPI(transport),
    preview: createFrontendPreviewAPI(transport, capabilities),
    raw: {
      http,
      transport,
    },
  };
};
