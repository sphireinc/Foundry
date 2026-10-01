# Media management

Foundry preserves authored media in its collection roots. Metadata lives beside
an original as `<filename>.meta.yaml`. Responsive output is derived build data,
not a replacement for the original.

## Responsive images and deterministic optimization

Add this to your site configuration to opt in:

```yaml
media:
  responsive_images: true
  widths: [320, 640, 960, 1280]
  jpeg_quality: 82
  require_alt: false
```

Run `foundry build` or `foundry assets build` to generate variants. You can also
run `foundry media optimize`, which refreshes the public assets and variants
without rendering pages. Omitted widths and quality use the values above.
Widths are sorted and deduplicated; upscaling is never performed. Configuration
accepts at most 16 widths from 1 through 8192 and JPEG quality from 1 through 100.

JPEG and static PNG files in enabled copied collections receive proportional
variants. JPEG pixels are oriented using EXIF before resizing; generated files
strip EXIF, including location data. PNG transparency is retained. At full size,
Foundry keeps the original URL when re-encoding would increase its size. Animated
GIF/PNG, WebP, AVIF and SVG are preserved and receive no automatic transforms.
Transforms reject images over 32 MiB or 40 megapixels before full decoding.

Generated files live under `public/_foundry/media/`. Names include a digest of
source bytes, transform size, quality and pipeline version. Identical inputs and
settings produce identical bytes and URLs with the same Go/library versions.
Replacing a source changes its variant names. Source files and sidecars are never
rewritten by optimization. The JSON index and variants are published with the
static site and served by Foundry at `/_foundry/media/`.

Rendered local `<img>` tags receive `srcset`, a default `sizes="100vw"`, and
intrinsic width/height when neither dimension is already specified. Existing
`srcset`, `sizes` and dimensions win. Theme authors should supply a `sizes` value
that describes their actual layout. Remote images are not downloaded or changed.
Regenerate public output after replacing or trashing source media; old public
copies remain accessible until that output is rebuilt. A clean `foundry build`
removes stale derived files; incremental asset builds retain older variants.

## Accessible image metadata

Use meaningful inline alt text in Markdown:

```markdown
![A green tree beside the entrance](media:images/tree.jpg)
```

You can also edit Alt Text in the admin media details form or save a sidecar:

```yaml
alt: A green tree beside the entrance
```

Saved metadata fills missing or empty inline alt text when rendering local media.
Nonempty inline descriptions take precedence. For intentionally decorative
images, select **Decorative image (empty alt text)** in the admin form, or set:

```yaml
decorative: true
```

This produces empty alt text and `role="presentation"` when no meaningful inline
alt is supplied. Hand-authored decorative HTML can use `alt=""` with
`role="presentation"`, `role="none"`, or `aria-hidden="true"`.

Set `media.require_alt: true` to reject rendered images without a description or
an explicit decorative declaration, and reject image metadata saves lacking
both. `foundry validate` checks Markdown image accessibility using the same
metadata rules. Final rendered output is checked too, including template/plugin
images. Uploads remain possible before metadata is completed. Existing bulk tag
editing remains available and preserves the decorative and technical metadata.

## Preview orphan cleanup and recover changes

The admin Media page has a **Media Audit** panel. Its preview reports Markdown
accessibility issues and potential orphan references without changing files.
The same report is available from the CLI:

```bash
foundry media audit
foundry media audit --json
```

The scanner checks current content (including drafts), project data, effective
configuration, active theme sources and plugin sources for `media:` references
and public URLs. It handles escaped URLs and considers references in frontmatter,
HTML, CSS and JSON. Sidecars, retained versions and trash are excluded as assets;
references that exist only in historical content do not protect current assets.

**Potential orphan does not mean safe to delete.** Dynamic URL construction,
external consumers and references outside the scanned roots may be invisible.
Inspect each candidate and its usage before choosing an action. The admin's
existing Delete action moves an individual file and its sidecar to Trash, and
Restore recovers them. No bulk or automatic orphan deletion is added.

The CLI also supports a preview followed by explicit recoverable trash:

```bash
foundry media trash media:images/unused.jpg
foundry media trash media:images/unused.jpg --apply
```

The apply command runs a fresh audit and refuses references that are known to be
used or absent from the candidate list. It moves the original and metadata to the
existing lifecycle Trash instead of permanently deleting them, then prints the
restore command for the retained path:

```bash
foundry media restore content/images/unused.trash.20261001T120000Z.jpg
foundry build
```

Use the actual retained path printed by the command. Restore also recovers the
sidecar. If a current file exists, the existing versioning workflow preserves it
before restoring. Rebuild after trash/restore to update published output.

## Admin API

`GET /__admin/api/media/audit` (or your configured admin prefix) requires
`media.read` and returns `assets`, `potential_orphans`, `accessibility`, and
`notes`. It performs no writes. `sdk/admin/media.js` exposes it as `media.audit()`.

The existing metadata API accepts `metadata.decorative` alongside `metadata.alt`.
It requires `media.write`, retains technical fields such as the content digest,
and uses existing metadata version history. Lifecycle actions retain their
existing `media.lifecycle` capability requirements.
