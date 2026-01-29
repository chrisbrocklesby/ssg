# ssg

A small, fast static site generator (SSG) written in Go.

- Templates: Go `html/template`
- Data: JSON files loaded into `.Site`, `.Page`, and `.Data`
- Static assets: copied from `src/static` to your output folder
- Dev server: optional watch + hot reload

## Install

### From source (local)

From this repo:

```sh
go test ./...
go build -o ~/bin/ssg ./cmd/ssg
```

Make sure `~/bin` is on your `PATH`.

> If you later publish this repo under a real module path (e.g. `github.com/you/ssg`), you can also use `go install github.com/you/ssg@latest`.

## Use as a Go package

You can embed SSG into an existing `net/http` server without shelling out to the CLI:

```go
stop, err := ssg.Attach(ctx, nil, ssg.Options{Root: "."})
if err != nil {
  log.Fatal(err)
}
defer stop()

http.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
  w.Write([]byte("Webserver is running"))
})

log.Fatal(http.ListenAndServe(":8081", nil))
```

## Quick start

Create a new project skeleton:

```sh
ssg init ./my-site
cd ./my-site
```

Build the site:

```sh
ssg build -src ./src -out ./dist
```

Serve the built output with hot reload:

```sh
ssg serve -src ./src -out ./dist -watch
```

## Commands

### `ssg init <projectDir>`

Creates a starter project structure under `<projectDir>/src`.

### `ssg build`

```sh
ssg build \
  -src ./src \
  -out ./dist \
  -cache ./.cache \
  -fetch-ttl 1h \
  -jobs 8 \
  -clean
```

Flags:
- `-src`: input folder (default `./src`)
- `-out`: output folder (default `./dist`)
- `-cache`: on-disk cache dir for `fetch` (default `./.cache`)
- `-nocache`: disable memory/disk fetch cache
- `-clean`: delete output folder before building
- `-fetch-ttl`: revalidate remote fetches after this duration (e.g. `30m`, `1h`)
- `-jobs`: parallelism for page builds

### `ssg serve`

Serves `-out` over HTTP.

```sh
ssg serve -src ./src -out ./dist -addr :8080 -watch -interval 500ms
```

- When `-watch` is enabled, it rebuilds on changes and triggers a browser reload.
- If a rebuild fails, HTML routes show an in-browser error page (useful during development).

## Project structure

Your source folder looks like:

```
src/
  site.json
  pages/
  layouts/
  partials/ *optional
  data/ *optional
  static/ *optional
```

### `src/site.json` → `.Site`

Global site data.

Example:

```json
{
  "title": "My Site",
  "baseURL": "https://example.com"
}
```

### `src/pages/**` → `.Page`

Pages are templates:
- `*.html` (Go template)
- `*.tmpl` (Go template)

Each page template has a JSON config next to it with the same stem:
- `pages/about.html` → `pages/about.json`
- `pages/feed.xml.tmpl` → `pages/feed.xml.json`

Page config is exposed as `.Page`.

Common page fields:
- `title`, `description`, etc (anything you want)
- `draft: true` to skip building the page
- `layout: "default"` (or another layout name)
- `layout: "none"` for standalone output
- `slug: "some/path"` to control the output URL
- `sitemap: false` to exclude an HTML page from `sitemap.xml`

### `src/layouts/*.(html|tmpl)` and `src/partials/**/*.(html|tmpl)`

Layouts and partials are parsed into the base template set.

- Default layout is `src/layouts/default.html` (but `.tmpl` is also supported)
- Layouts must define templates like `{{ define "default" }}`
- Partials should be named like `{{ define "partials/header" }}`

### `src/data/**/*.json` → `.Data`

All JSON files under `src/data` are loaded and made available under `.Data` by path.

Example:

- `src/data/nav.json` becomes `.Data.nav`
- `src/data/blog/posts.json` becomes `.Data.blog.posts`

Data path collisions are errors (e.g. `data/a.json` and `data/a/b.json`).

### `src/static/**` → copied to output

Everything under `src/static` is copied to the output folder root.

Example:

- `src/static/css/style.css` → `dist/css/style.css`

## Rendering rules

### Default layout + `content`

For normal HTML pages, you typically render through a layout template like `default`.

- If the page file defines `{{ define "content" }}`, that block is used.
- If it does not, the entire page file is treated as the `content` block automatically.

### Standalone pages (`layout: "none"`)

If a page has `layout: "none"`, it renders standalone using `__page__`.

- If the page defines `{{ define "__page__" }}`, that block is used.
- Otherwise the whole file is wrapped as `__page__`.

### Hardening: allowed `define`s

Pages may only define:
- `content`
- `__page__`

This prevents pages from introducing unexpected template definitions.

## Output paths

- `pages/index.html` → `dist/index.html`
- `pages/about.html` → `dist/about/index.html`
- `pages/blog/index.html` → `dist/blog/index.html`
- `pages/feed.xml.tmpl` → `dist/feed.xml`

If `.Page.slug` is set for an HTML page:
- `slug: ""` → `dist/index.html`
- `slug: "docs"` → `dist/docs/index.html`
- `slug: "docs/getting-started"` → `dist/docs/getting-started/index.html`

## Template data

Templates execute with:

- `.Site`: the object from `src/site.json` (plus any modifications like `fetch` results)
- `.Page`: the object from the page’s `*.json` config
- `.Data`: the object tree built from `src/data/**/*.json`

## Template functions

The base template registers a few helpers:

- `absURL` – prefix a path with `.Site.baseURL` (if set)
  - `{{ absURL "/css/style.css" }}`
- `safeHTML` – render a string as unescaped HTML (use with care)
  - `{{ safeHTML .Page.bodyHTML }}`
- `date` – format a time value
  - `{{ date "2006-01-02" .Page.date }}`
  - `{{ date "15:04" }}` (formats current time)
  - Go uses a reference time layout (not `YYYY/MM/DD`).
- `json` – JSON encode a value for embedding into HTML/JS
  - `<script>window.__DATA__ = {{ json .Data }};</script>`

## Fetch (HTTP data at build time)

You can fetch remote data in `site.json` or in a page config JSON using a `fetch` object.

Example in `site.json`:

```json
{
  "title": "My Site",
  "fetch": {
    "posts": { "url": "https://example.com/posts.json", "as": "json" }
  }
}
```

Then in templates:

- `{{ index .Site.fetch "posts" }}`

Fetch behavior:
- In-memory + on-disk caching (under `-cache`)
- TTL-based revalidation via ETag/Last-Modified when available
- In-flight request de-duplication during parallel builds

## Sitemap

If `.Site.baseURL` is set and you generated HTML pages, `ssg` writes:

- `dist/sitemap.xml`

Notes:
- Only `.html` output is included.
- `<lastmod>` uses the page template file’s mtime.
- Per page opt-out: set `"sitemap": false` in that page’s JSON.

## Using with a Go HTTP server

A simple pattern is:

- run `ssg build` as part of your deploy/build step
- serve `dist/` as static files
- keep `/api/*` routes in your Go server

(If you want, you can also shell out to `ssg build` at app startup in development.)
