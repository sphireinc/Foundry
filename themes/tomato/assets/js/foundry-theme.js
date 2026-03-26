import { createFrontendClient } from '/__foundry/sdk/frontend/index.js';

// The Tomato theme uses the official Frontend SDK instead of coupling directly
// to internal API payloads.
const client = createFrontendClient({ mode: 'auto' });

const bootstrap = async () => {
  const [capabilities, site, current, navigation, route] = await Promise.all([
    client.capabilities.get(),
    client.site.getInfo(),
    client.content.getCurrent().catch(() => null),
    client.navigation.get('main').catch(() => []),
    client.routes.current().catch(() => null),
  ]);

  const state = {
    capabilities: capabilities.raw,
    site,
    current,
    navigation,
    route,
  };

  window.FoundryFrontend = {
    client,
    state,
  };

  document.documentElement.dataset.foundrySdk = 'frontend-v1';
  document.documentElement.dataset.foundryTheme = 'tomato';
  document.dispatchEvent(new CustomEvent('foundry:ready', { detail: state }));
};

void bootstrap();
