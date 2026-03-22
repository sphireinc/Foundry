import { createFrontendClient } from '/__foundry/sdk/frontend/index.js';

// The default theme talks to Foundry through the official Frontend SDK which we ipport above
// instead of hard-coding fetch calls to internal endpoints.
const client = createFrontendClient({ mode: 'auto' });

const bootstrap = async () => {
  // Load capability info, site metadata, current content, navigation, and route context.
  const [capabilities, site, current, navigation, route] = await Promise.all([
    client.capabilities.get(),
    client.site.getInfo(),
    client.content.getCurrent().catch(() => null),
    client.navigation.get('main').catch(() => []),
    client.routes.current().catch(() => null),
  ]);

  // creaet our state
  const state = {
    capabilities: capabilities.raw,
    site,
    current,
    navigation,
    route,
  };

  // Expose the runtime in window
  window.FoundryFrontend = {
    client,
    state,
  };

  // Mark the page with the active SDK contract and emit a ready event so
  // other scripts can wait for Foundry data without coupling to this file
  document.documentElement.dataset.foundrySdk = 'frontend-v1';
  document.dispatchEvent(new CustomEvent('foundry:ready', { detail: state }));
};

// Fire the bootstrap immediately onlaod
void bootstrap();
