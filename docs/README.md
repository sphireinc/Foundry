# Documentation website

GitHub Pages publishes the `docs/` directory after the main-branch coverage job
succeeds. The shared styling and navigation live in `site.css`, `theme-toggle.js`,
and the HTML pages. The homepage is also the header source for generated guides.

Edit the canonical Markdown guides for editorial workflows, media management,
site search, extension authoring, installation updates, content bundles, and the
managed runtime boundary. Then regenerate their styled pages from the repository root:

```sh
go run ./scripts/cmd/docs-build
python3 -m http.server 8000 --directory docs
```

Open <http://localhost:8000/>. No Foundry server or site build is required.
Commit both the Markdown and generated HTML when changing a guide. CI regenerates
the pages before uploading the Pages artifact. Existing hand-authored guides stay
hand-authored; update their navigation alongside `docs/index.html` when adding links.

The generated pages use local assets and work under the GitHub Pages `/Foundry/`
prefix. Links to other rendered guides stay within the documentation site; links
to repository source files point to GitHub. Preview coverage uses the checked-in
placeholder until CI generates the current report.
