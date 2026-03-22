import { createAdminClient } from '../admin/index.js';

const admin = createAdminClient({ baseURL: '/__admin' });

const session = await admin.session.get();
const capabilities = await admin.capabilities.get();
const documents = await admin.documents.list({ include_drafts: 1, q: 'launch' });

console.log({
  authenticated: session.authenticated,
  canManagePlugins: capabilities.has('plugins.manage'),
  documentCount: documents.length,
});
