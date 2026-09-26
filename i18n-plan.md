# Issue #122: Admin and Standard UI Localization Plan

## Purpose

Add first-class interface translation to the built-in admin console and the standard public theme. The current issue calls out customers who do not know English and proposes wrapping captions in a `_t()`-style translator.

## Scope and language behavior

- English remains the source language and fallback so existing installations and untranslated copy continue to work.
- Add a Spanish (`es`) catalogue as the first complete non-English UI translation. Keep the catalogue format open to further locale files without adding a runtime or build dependency.
- Keep interface locale distinct from content configuration. Public default-theme copy follows the current page's language (`ViewData.Lang`) and falls back from regional tags (for example, `es-MX`) to the base language and then English.
- The default admin console offers English and Spanish. It starts from a saved admin choice, otherwise the browser's preferred supported language, otherwise English. The selection is stored in browser local storage and does not alter `default_lang` or document content.
- Only Foundry-authored interface labels, helper text, prompts, empty states, accessibility labels, and built-in notices are translated. User-authored content, configured site name/title, plugin/theme-provided labels, filenames, API values, and code examples remain unchanged.

## Architecture

1. Add a JSON message catalogue in `internal/i18n` and embed/read it through the existing Go i18n package. Provide normalized locale lookup, regional-to-base fallback, English fallback, a stable language list, and JSON serialization for the admin bootstrap.
2. Add a Go template translator function and use it in every built-in `themes/default/layouts` template where Foundry supplies visible UI copy. Pass the page language explicitly; do not translate document fields.
3. Add an admin `_t()` module backed by the same catalogue JSON passed by `internal/admin/ui.Manager` to the default admin page. Use safe JSON-in-HTML bootstrapping and locale normalization; preserve English behavior if the catalogue is unavailable.
4. Add an accessible language selector to the default admin shell. Re-render the current view after changing locale, persist the choice, and keep the choice separate from server/site content settings.
5. Wrap user-facing static strings throughout the default admin entry point and its core, view, editor, and event modules. Use interpolation for dynamic notices while preserving HTML escaping and leaving API/user-provided values untouched.
6. Update tests, including Go locale normalization/fallback/catalog tests, public template rendering tests for translated and fallback copy, and admin bootstrap/selector or JavaScript tests appropriate to the existing test harness.

## Acceptance criteria

- A Spanish browser preference selects Spanish on first use of the default admin console; an explicit admin selection persists and overrides the browser preference.
- Every built-in admin surface (navigation, login, overview, content/editor, media, history/trash, users/sessions/audit, settings, extensions/plugins/themes, operations/diagnostics/debug, keyboard/help, prompts, and empty/error notices) renders English or Spanish interface copy through `_t()`.
- The default public theme renders its built-in interface text in the page language for English and Spanish documents. Unsupported languages and missing keys use English/source text, and regional tags fall back to their base locale.
- User content and configured values are not translated or corrupted; `default_lang` remains a content-language setting.
- Existing custom themes and custom admin themes retain their behavior. The built-in default admin has an English fallback if a theme does not include localization bootstrap data.
- Focused Go tests and the repository's available JavaScript/e2e checks pass; `git diff --check` is clean.

## Implementation order

1. Catalogue and locale fallback API with tests.
2. Go public-template translator and Spanish translations, with rendering tests.
3. Admin bootstrap, `_t()`, locale selector/persistence, and tests.
4. Translate all built-in admin interface strings, grouped by module.
5. Run focused checks, inspect remaining untranslated Foundry-authored UI copy, then stage only issue #122 files. Do not commit.

## Risks and mitigations

- **Missed hard-coded captions:** search all first-party default admin modules and default public layouts after replacement; deliberately exclude vendored Quill, user data, and protocol values.
- **Content/interface language confusion:** keep browser/admin locale and page `.Lang` separate from `Config.DefaultLang`; add tests proving the distinction.
- **Unsafe HTML bootstrapping or interpolation:** rely on `html/template` escaping for attributes, use text substitution in JS, and continue escaping values before inserting rendered HTML.
- **Incomplete translations:** English remains a per-key fallback; tests ensure catalogue coverage for the built-in English keys while Spanish copy is filled in for the supported surfaces.
