package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/chrisbrocklesby/ssg/internal/ssg"
)

func Main(args []string) int {
	if len(args) < 2 {
		_ = usage()
		return 2
	}

	switch args[1] {
	case "init":
		return exit(initCmd(args[2:]))
	case "build":
		return exit(buildCmd(args[2:]))
	case "serve":
		return exit(serveCmd(args[2:]))
	case "-h", "--help", "help":
		_ = usage()
		return 0
	default:
		_ = usage()
		return 2
	}
}

func exit(err error) int {
	if err == nil {
		return 0
	}
	if !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "error:", ssg.HumanizeError(err))
	}
	return 1
}

func usage() error {
	_, _ = fmt.Fprint(os.Stderr, `ssg

Commands:
  ssg init <projectDir>

	ssg build [-src ./src] [-out ./dist] [-cache ./.cache] [-nocache] [-clean] [-fetch-ttl 1h] [-jobs N]
					 [-extract-inline-styles] [-inline-styles-out css/inline.css]

  ssg serve [-src ./src] [-out ./dist] [-cache ./.cache] [-nocache] [-clean] [-fetch-ttl 1h] [-jobs N]
           [-addr :8080] [-watch] [-interval 500ms]
					 [-extract-inline-styles] [-inline-styles-out css/inline.css]

Pages:
  - Templates under src/pages can be:
      *.tmpl  (Go template)
      *.html  (Go template)
  - Page config is next to the template, same stem:
      pages/about.tmpl      -> pages/about.json
      pages/about.html      -> pages/about.json
      pages/feed.xml.tmpl   -> pages/feed.xml.json

Behavior:
  - If a non-standalone page doesn't define {{ define "content" }}, the whole file is treated as content.
  - page.json: { "layout": "none" } renders standalone via "__page__".
  - Hardening: pages may only define "content" and/or "__page__" and may not introduce new template names.

Fetch:
  - Cached in memory + on disk under -cache
  - -fetch-ttl controls freshness; stale entries are revalidated using ETag/Last-Modified when available
  - In-flight de-duplication prevents repeated network fetches under parallel builds

Sitemap:
  - Writes dist/sitemap.xml for generated HTML pages
  - Requires site.json.baseURL (absolute base URL)
  - Includes <lastmod> from page template file mtime
  - Per-page opt-out: { "sitemap": false }

Conventions:
  ./src/site.json                 -> .Site (map)
  ./src/data/**/*.json            -> .Data (map, path-based)
  ./src/static/**                 -> copied to out root
	./src/layouts/default.html       -> default layout (define "default"; also supports default.tmpl)
	./src/partials/**/*.(html|tmpl)  -> partials (define "partials/...")
`)
	return nil
}
