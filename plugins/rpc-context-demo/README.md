# RPC Context Demo

This plugin is a minimal reference implementation for Foundry's out-of-process
RPC plugin runtime.

It exists to demonstrate:
- how a plugin can run with `runtime.mode: rpc` instead of in-process
- how the host performs the RPC handshake
- how an RPC plugin implements the `OnContext` hook over stdin/stdout
- how render-time context data can be added without running plugin code inside
  the main Foundry process

What it does:
- starts the demo RPC server from [cmd/server/main.go](./cmd/server/main.go)
- advertises support for the `OnContext` hook
- injects a `rpc_context_demo` object into the render context with:
  - `enabled`
  - `title`
  - `page_id`

This is not meant to be an end-user feature plugin. It is primarily:
- a developer example
- a reference for future RPC plugin authors
- a proof that the current RPC host/process boundary works for the context hook

The plugin manifest is in [plugin.yaml](./plugin.yaml).
