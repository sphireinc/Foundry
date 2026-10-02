import { mountSearch } from './search-ui.js';

// Optional bootstrap for themes. Import search-ui.js alone for manual mounting.
for (const root of document.querySelectorAll('[data-foundry-search]')) mountSearch(root);
