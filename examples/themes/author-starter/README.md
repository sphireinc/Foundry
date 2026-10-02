# author-starter

A local-assets-only frontend theme starter. Initializing it does not switch the site's active theme.

From the site root:

```sh
foundry theme validate author-starter --security --csp
foundry theme switch author-starter
foundry serve
```

Use a copied site or config overlay for experiments. Edit layouts and assets while serving; run validation and a production `foundry build` before deployment. See docs/extension-authoring.md for the full workflow.

The manifest denies remote assets and frontend requests by default. Declare exact origins by asset category before adding external resources. The base layout exposes plugin slots; `.Data` holds plugin context and `.Page.Fields` holds document fields.
