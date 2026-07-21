# Content Bundles

`foundry content export site.zip` creates a portable, standalone ZIP package.
It does not alter the normal Foundry project layout. The export contains a
`foundry-content-manifest.yaml` and a `content/` directory with supported
documents and media files.

The manifest records the format version, selected theme, and a SHA-256 digest
for every included file. Admin user records are never included. Runtime
configuration, session data, plugins, backups, and generated public output are
also outside the package.

Import with:

```text
foundry content import bundle site.zip
```

Foundry extracts the package into a temporary directory, rejects unsafe paths,
symbolic links, unsupported formats, and checksum mismatches, then verifies the
declared theme. The destination must already have that theme installed and
selected. Only after validation succeeds does Foundry atomically replace the
existing `content/` directory. If replacement fails, the previous directory is
restored. Markdown and WordPress imports remain available for source-format
migration and do not use this package format.

`examples/starter-content/consulting/content/` is a professional,
provider-neutral first-site template. Copy it into a new project, replace the
example contact address, then export it when you need a reusable bundle.
