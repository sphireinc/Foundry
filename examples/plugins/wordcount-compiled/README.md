# wordcount-compiled

Adds a word count under `.Data.wordcount_compiled.words`. No network or filesystem access is needed by the hook.

Keep this directory inside `plugins/` in a Foundry source checkout; compiled plugins import Foundry internal packages and cannot be built as standalone external modules. From the checkout root:

```sh
go test ./plugins/wordcount-compiled
foundry plugin validate wordcount-compiled --security
foundry plugin enable wordcount-compiled
foundry plugin sync
go build -o ./foundry ./cmd/foundry
```

Restart the rebuilt Foundry binary after changing source. Compiled plugins run with the host process's privileges.

From the site root, run `foundry plugin validate wordcount-compiled --security` before enabling. Validation checks metadata and detected permissions; it does not compile, launch, or prove that the plugin is safe. Enabling may require `--approve-risk` after inspecting `foundry plugin security wordcount-compiled`. Initializing this starter does not enable it.

The RPC default sandbox is a trusted development mode, not an OS isolation guarantee. For deployment, inspect the strict-profile requirements in the plugin documentation.
