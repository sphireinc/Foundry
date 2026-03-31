import { createHttpClient } from '../core/http.js';
import { FoundryUnsupportedError } from '../core/errors.js';
import { createAdminAuditAPI } from './audit.js';
import { createAdminBackupsAPI } from './backups.js';
import { createAdminCapabilitiesAPI } from './capabilities.js';
import { createAdminDocumentsAPI } from './documents.js';
import { createAdminMediaAPI } from './media.js';
import { createAdminOperationsAPI } from './operations.js';
import { createAdminPluginsAPI } from './plugins.js';
import { createAdminSessionAPI } from './session.js';
import { createAdminSettingsAPI } from './settings.js';
import { createAdminStatusAPI } from './status.js';
import { createAdminThemesAPI } from './themes.js';
import { createAdminUpdatesAPI } from './updates.js';
import { createAdminUsersAPI } from './users.js';

// createAdminClient constructs the official JavaScript client for Foundry's
// authenticated admin surface.
//
// The client exposes domain-oriented modules rather than raw endpoints so
// admin themes, alternate admin frontends, and plugin UIs can target a stable
// contract.
export const createAdminClient = ({
  baseURL = '/__admin',
  fetch: fetchImpl,
  headers = {},
  getSession,
} = {}) => {
  const sessionStore = { current: null };
  const readSession = () =>
    typeof getSession === 'function' ? getSession() : sessionStore.current;
  const http = createHttpClient({
    baseURL,
    fetchImpl,
    headers,
    prepareRequest({ safe, headers: requestHeaders }) {
      const session = readSession();
      if (!safe && session?.csrf_token) {
        requestHeaders['X-Foundry-CSRF-Token'] = session.csrf_token;
      }
    },
  });

  const session = createAdminSessionAPI(http, sessionStore);
  const capabilities = createAdminCapabilitiesAPI(http, sessionStore);
  return {
    session,
    identity: {
      async getCurrentUser() {
        const current = await session.get();
        if (!current?.authenticated) return null;
        return {
          username: current.username,
          name: current.name,
          email: current.email,
          role: current.role,
          capabilities: current.capabilities || [],
        };
      },
    },
    capabilities,
    status: createAdminStatusAPI(http),
    documents: createAdminDocumentsAPI(http),
    media: createAdminMediaAPI(http),
    settings: createAdminSettingsAPI(http),
    users: createAdminUsersAPI(http),
    themes: createAdminThemesAPI(http),
    plugins: createAdminPluginsAPI(http),
    operations: createAdminOperationsAPI(http),
    audit: createAdminAuditAPI(http),
    backups: createAdminBackupsAPI(http),
    updates: createAdminUpdatesAPI(http),
    extensions: {
      async getAdminExtensions() {
        const capabilitySet = await capabilities.get();
        if (!capabilitySet.feature('plugin_admin_registry')) {
          return { pages: [], widgets: [], slots: [], settings: [] };
        }
        return http.get('/api/extensions');
      },
    },
    sync: {
      async getStatus() {
        throw new FoundryUnsupportedError('Sync and remote storage APIs are not implemented yet');
      },
    },
    raw: http,
  };
};

createAdminClient.prototype = {};
