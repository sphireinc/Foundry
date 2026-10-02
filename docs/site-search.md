# Built-in site search

Foundry ships site search for live and static deployments. No plugin or external
search service is required. The default theme links to the current language's
search page and progressively enhances its search form with the frontend SDK.

## Search contract

```js
import { createFrontendClient } from '/__foundry/sdk/frontend/client.js';
const client = createFrontendClient({ mode: 'auto' });
const results = await client.search.query('alpha', { lang: 'en', limit: 20 });
```

`GET /__foundry/api/search?q=alpha&lang=en&type=page&limit=20` and
`client.search.query('alpha', { lang: 'en', type: 'page', limit: 20 })` return:

```json
{ "query": "alpha", "items": [], "total": 0, "limit": 20 }
```

- `q` is trimmed and lowercased. An empty query returns no results and prompts the
  visitor to search; it does not list the whole site.
- `lang` selects an exact content language. If omitted, the site's default
  language applies. Search pages use their route language (`/search/`,
  `/es/search/`, and so on), including when submitted without JavaScript.
- `type` optionally selects an exact content type, such as `page` or `post`.
  An unknown language/type returns no matches, rather than broadening the search.
- `limit` defaults to 20; positive integers are capped at 100. Invalid or
  nonpositive values use the default. `total` counts all matches before limiting.
- A query matches a contiguous phrase in the title, summary, content, or URL.
  Weights are 6, 4, 2, and 1 respectively; matching fields contribute once each.
  Ties use title, then URL, in Unicode ordinal order, independently of browser locale.
- Matching is case-insensitive, retains accents, and has no stemming or fuzzy
  matching. `café` matches `CAFÉ`; `cafe` does not match `café`.
- Snippets prefer the summary, otherwise normalized content, otherwise title;
  at most 180 Unicode characters are shown with ellipses when truncated.
  Body-only snippets include the matching phrase where possible.

## Live and static deployment

Live search renders HTML on the server and exposes the JSON API. The form remains
usable when JavaScript is disabled. Frontend enhancement provides debounced
searching, keyboard submission, loading/error/empty states, accessible result
announcements, and protection against stale asynchronous results.

`foundry build` emits both `public/search.json` and
`public/__foundry/search.json`, plus the SDK and language-specific search pages.
Publish the **whole public directory**, including `__foundry/`, on a static host.
Static hosts do not evaluate URL query strings: the browser enhancement reads
`?q=` and queries the static index. JavaScript is required to search on those hosts.

The SDK's `auto` mode tries the live API and falls back to static artifacts when
it is unavailable. Use `mode: 'static'` when the deployment is known to be static,
or `mode: 'api'` to report API failures instead of falling back. A static index is
loaded and parsed once per client/page session, with concurrent requests sharing
one load. A failed load is retried on the next search. A reload obtains a new index
subject to the host's HTTP cache policy; invalidate or revalidate search JSON when
deploying, rather than caching it indefinitely under its stable filename.

Only published content enters public search, including when the input graph was
loaded for draft preview. Draft, review, archived, future-scheduled and expired
content is excluded. Static indexes are snapshots at build time: scheduled
publication/unpublication requires a rebuild and deployment at the relevant time.
Do not publish preview build artifacts as a substitute for a production build.

The index contains full normalized searchable content for every public document,
including all languages. Result limits bound returned matches, not index size.
Download/parsing and each query are linear in the index size; ranking sorts matches.
Enable HTTP compression, measure index sizes on your site, and consider a specialized
search backend if a very large content library outgrows an in-browser JSON index.

## Other themes

Use the SDK directly, or load the reusable enhancement from
`/__foundry/sdk/frontend/search-page.js`. For the enhancement, provide:

```html
<section data-foundry-search data-search-lang="en">
  <form action="/search/" method="get" role="search">
    <label for="q">Search</label>
    <input id="q" name="q" type="search" />
    <button type="submit">Search</button>
  </form>
  <p data-search-status role="status" aria-live="polite"></p>
  <div data-search-results></div>
</section>
<script type="module" src="/__foundry/sdk/frontend/search-page.js"></script>
```

Render server results inside the results container for progressive enhancement.
Use `SearchURL`, `SearchQuery`, `SearchType`, `SearchTotal`, and `SearchLimit` from
Foundry's template context. Optional form control `name="type"` enables filtering.
Set `data-search-prompt`, `data-search-loading`, `data-search-empty`,
`data-search-error`, and `data-search-count` for localized messages.

For a custom client or debounce interval, import `mountSearch` from
`/__foundry/sdk/frontend/search-ui.js` instead of loading `search-page.js`, and pass
`{ client, debounce: 200 }`. Result titles and snippets are inserted as text;
remote/script result URLs are rejected. Keep the SDK and data paths accessible
and permit same-origin scripts and requests in your theme's security policy.
The shipped theme assumes deployment at the origin root. For a site mounted under
a path prefix, adapt theme asset/form URLs and supply the SDK's `baseURL` through
a custom client.

## Verification

Run `go test ./internal/sitesearch ./internal/platformapi ./internal/renderer`
for the shared Go contract and `npm run test:search` for browser acceptance on an
isolated live server and a plain static file server. The browser suite uses Google
Chrome by default; set `FOUNDRY_SEARCH_BROWSER_CHANNEL` to another installed
Playwright channel if necessary. Its fixture site uses temporary content and output
and does not change the checked-out site's configuration or data.
