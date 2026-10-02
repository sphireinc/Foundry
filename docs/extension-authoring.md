# Plugin and theme authoring

Foundry already provides theme scaffolding, manifest/security validation, plugin
security reports, field-contract migration, and local live reload. Use those
commands together before installing or enabling an extension on a production site.

## Existing tools and new entry points

| Task                     | Command                                            | What it checks or changes                                                                  |
| ------------------------ | -------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| New RPC plugin (default) | `foundry plugin init my-plugin`                    | Creates source, tests, module, manifest, and README; does not enable                       |
| New compiled plugin      | `foundry plugin init my-plugin --runtime compiled` | Creates a package for the Foundry source module; does not enable                           |
| New frontend theme       | `foundry theme init my-theme`                      | Uses the existing `theme scaffold` generator; does not switch themes                       |
| Plugin validation        | `foundry plugin validate my-plugin --security`     | Metadata, runtime contract, detected permissions                                           |
| Plugin security review   | `foundry plugin security my-plugin`                | Declared/detected capabilities and runtime enforcement                                     |
| Theme validation         | `foundry theme validate my-theme --security --csp` | Layouts, partials, template references, field contracts, external assets and requests, CSP |
| Theme security review    | `foundry theme security my-theme`                  | Origins, policy mismatches, remediation and effective CSP                                  |
| Field-contract migration | `foundry theme migrate field-contracts`            | Existing migration for document/shared field contracts                                     |
| Compiled registration    | `foundry plugin sync`                              | Generates imports for enabled compiled plugins                                             |

Run commands from a site root with its configuration available. Both initializers
use the configured `plugins_dir` or `themes_dir`. Names for plugin starters begin
with a lowercase letter and contain lowercase letters, digits, hyphens, or
underscores. Existing destinations are refused, including symlinks. Initialization
does not execute generated code, fetch dependencies, grant risk approval, enable a
plugin, or switch an active theme.

## Choose the plugin runtime

Use **RPC** when authoring an extension independently of Foundry's source tree.
The starter is a separate Go module using the public
[`sdk/pluginrpc`](../sdk/pluginrpc/protocol.go) API, pinned to Foundry v1.4.6.
It implements a page-context hook and includes a word-count regression test.

```sh
foundry plugin init wordcount-rpc
cd plugins/wordcount-rpc
go mod tidy
go test ./...
mkdir -p bin
go build -o bin/wordcount-rpc ./cmd/server
cd ../..
foundry plugin validate wordcount-rpc --security
foundry plugin security wordcount-rpc
# After inspecting the report and trusting this code:
foundry plugin enable wordcount-rpc --approve-risk
foundry serve
```

The manifest executes `./bin/wordcount-rpc` relative to the plugin directory.
Build for the deployment host's OS and architecture. Rebuild the binary and
restart Foundry after changing RPC source; theme live reload does not rebuild
plugin executables. Keep stdout exclusively for JSON-RPC, and send logs to stderr.
`go mod tidy` is a separate, explicit dependency-download step; initialization is
local-only. Update the SDK pin deliberately when adopting newer protocol APIs.

The default RPC sandbox is **trusted development mode**, not an OS isolation
guarantee. The context hook needs only document payload and render-context
permissions; network, filesystem writes, and child-process permissions stay off.
For deployment, review the strict sandbox's platform and artifact requirements in
[Theme and plugin security](../README.md#theme-and-plugin-security). Strict mode
requires a prebuilt executable inside the plugin artifact and can be unavailable
on some hosts. It fails closed when unsupported. Do not replace the runtime command
with `go run` for strict execution.

Use **compiled** plugins for deeper integration in a Foundry source checkout.
These import `internal/plugins` and `internal/renderer`, so an external standalone
module cannot compile them. They run with the Foundry process's privileges.

```sh
foundry plugin init wordcount-compiled --runtime compiled
go test ./plugins/wordcount-compiled
foundry plugin validate wordcount-compiled --security
foundry plugin security wordcount-compiled
foundry plugin enable wordcount-compiled
foundry plugin sync
go build -o ./foundry ./cmd/foundry
./foundry serve
```

Use `--approve-risk` only when the enable command requests approval and after
reviewing the report. Restart the rebuilt Foundry binary after source changes.
The generator converts hyphens to underscores for the Go package/context key,
while the manifest, registration, and directory keep the original plugin name.
Both starters write the word count under `.Data.<context_key>.words` and handle
pages without a document payload.

## Theme development loop

```sh
foundry theme init my-theme
foundry theme validate my-theme --security --csp
# In a development copy of the site:
foundry theme switch my-theme
foundry serve
```

The initializer is an alias for `theme scaffold`, retaining its layouts, partials,
plugin slots, local CSS, and live reload integration. Its generated README
explains the next steps. The security manifest denies external assets and
frontend requests by default. When adding remote resources, declare the exact
origin in the correct category, enable the corresponding policy, and validate
again. Existing theme-security diagnostics distinguish missing declarations,
disabled policy, and unsupported network resources; follow their specific hints.

A template parse pass cannot prove every template will execute correctly for
real content. Render pages, posts, index/list pages, translations, and documents
with absent optional fields. Run a production `foundry build` before deployment.
For experiments without switching the real site, use a copied site or a
`--config-overlay` with separate content/data/public paths and the test theme.
See [configuration examples](../content/config/example.site.yaml).

## Diagnostics and fix hints

Plugin validation now supplies a next step on failure, including inspecting the
security report before declaring permissions, adding missing source/registration,
or fixing the manifest field named in the error. A security mismatch prints the
detected-capability details even when normal validation fails early. Theme
structural errors provide hints for missing layouts/partials and SDK/name
mismatches, alongside the existing security hints.

Use `foundry plugin validate --security` without a name to also check dependency relationships across enabled plugins.

Validation **does not compile or execute plugins**. Passing validation is not a
loadability or safety guarantee. Run tests and builds separately, then exercise
the enabled extension in an isolated development site. Foundry does not add broad
permissions automatically to make validation pass. Remove unintended capabilities
or declare only the intended permissions, review the resulting trust boundary,
and rerun validation. Initializers never overwrite an existing extension to
repair it; generate a fresh starter under another name for comparison.

## Runnable examples

See [examples](../examples/README.md) for a compiled word-count plugin, a standalone
RPC equivalent, and a local-assets-only theme that displays either plugin's
context. None is enabled or imported into the shipped runtime automatically.
The existing [RPC context demo](../plugins/rpc-context-demo/README.md) additionally
shows HTML slots and after-render hooks; the built-in plugins and default themes
remain fuller examples of Foundry's APIs.
