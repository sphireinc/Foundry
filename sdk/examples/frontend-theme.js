import { createFrontendClient } from '../frontend/index.js';

const frontend = createFrontendClient({ mode: 'auto' });

const site = await frontend.site.getInfo();
const current = await frontend.content.getCurrent();
const mainNav = await frontend.navigation.get('main');
const relatedPosts = await frontend.collections.list('post', {
  lang: current?.lang,
  taxonomy: 'tags',
  term: current?.taxonomies?.tags?.[0],
  page_size: 5,
});

console.log({
  site: site.title,
  current: current?.title,
  navigationItems: mainNav.length,
  relatedPosts: relatedPosts.items.length,
});
