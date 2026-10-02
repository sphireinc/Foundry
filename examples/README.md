# Extension author examples

These examples are disabled by default. They demonstrate the starters produced
by `plugin init` and `theme init`, plus a theme partial consuming their context.
Use a development copy of a site and inspect permissions before enabling them.

- [Compiled word count](plugins/wordcount-compiled/README.md): registration,
  context mutation, and a runnable Go test inside Foundry's module.
- [RPC word count](plugins/wordcount-rpc/README.md): independent module using the
  public SDK, a binary entry point, handshake, context hook, and tests.
- [Author starter theme](themes/author-starter/README.md): required layouts,
  plugin slots, local CSS, and a `wordcount` partial used by pages and posts.

From the Foundry checkout, test the examples without changing site configuration:

```sh
go test ./examples/plugins/wordcount-compiled
(cd examples/plugins/wordcount-rpc && go mod tidy && go test ./...)
```

Copy each desired plugin directory under your site's configured `plugins_dir`,
and the theme directory under `themes_dir`. For the RPC plugin, run these commands
from its copied directory before enabling it:

```sh
mkdir -p bin
go build -o bin/wordcount-rpc ./cmd/server
```

Compiled plugins must live under `plugins/` inside a Foundry source checkout;
copying them into a binary-only site does not add their code to that binary.
Validate and enable using the steps in
[Extension authoring](../docs/extension-authoring.md), then rebuild Foundry for a
compiled plugin. Switch the copied `author-starter` theme in the development site
and render a page or post. It displays whichever of `wordcount-rpc` and
`wordcount-compiled` is enabled, and works without either plugin.

For local SDK development, the RPC example can use a temporary module replacement
pointing to your checkout. Do not commit a developer-specific replacement or
ship it as part of an installed artifact. The checked-in example tests use the
released SDK pin.
