# wordcount-rpc

Adds a word count under `.Data.wordcount_rpc.words`. No network or filesystem access is needed by the hook.

From this plugin directory (Go 1.26 or newer):

```sh
go mod tidy
go test ./...
mkdir -p bin
go build -o bin/wordcount-rpc ./cmd/server
```

The manifest runs the built binary, not `go run`. Rebuild it after source changes, then restart Foundry. Keep stdout reserved for JSON-RPC; send diagnostics to stderr.

From the site root, run `foundry plugin validate wordcount-rpc --security` before enabling. Validation checks metadata and detected permissions; it does not compile, launch, or prove that the plugin is safe. Enabling may require `--approve-risk` after inspecting `foundry plugin security wordcount-rpc`. Initializing this starter does not enable it.

The RPC default sandbox is a trusted development mode, not an OS isolation guarantee. For deployment, inspect the strict-profile requirements in the plugin documentation.
