package ssg

import (
	"fmt"
	"os"
	"path/filepath"
)

func Init(c InitConfig) error {
	if c.SrcDir == "" {
		return fmt.Errorf("missing SrcDir")
	}

	mustMkdir(filepath.Join(c.SrcDir, "pages"))
	mustMkdir(filepath.Join(c.SrcDir, "layouts"))
	mustMkdir(filepath.Join(c.SrcDir, "partials"))
	mustMkdir(filepath.Join(c.SrcDir, "data"))
	mustMkdir(filepath.Join(c.SrcDir, "static", "css"))

	writeIfMissing(filepath.Join(c.SrcDir, "site.json"), `{
  "title": "New Site",
  "baseURL": "http://example.com"
}
`)
	writeIfMissing(filepath.Join(c.SrcDir, "data", "nav.json"), `{
  "items": [
    { "label": "Home", "url": "/" }
  ]
}
`)
	writeIfMissing(filepath.Join(c.SrcDir, "layouts", "default.html"), `{{ define "default" -}}
<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>{{ if .Page.title }}{{ .Page.title }} - {{ end }}{{ .Site.title }}</title>
    <link rel="stylesheet" href="/css/style.css" />
  </head>
  <body>
    {{ template "partials/header" . }}
    <main>{{ template "content" . }}</main>
    {{ template "partials/footer" . }}
  </body>
</html>
{{- end }}
`)
	writeIfMissing(filepath.Join(c.SrcDir, "partials", "header.html"), `{{ define "partials/header" -}}
<header>
  <nav>
    {{ if .Data.nav }}
      {{ range .Data.nav.items }}
        <a href="{{ .url }}">{{ .label }}</a>
      {{ end }}
    {{ end }}
  </nav>
</header>
{{- end }}
`)
	writeIfMissing(filepath.Join(c.SrcDir, "partials", "footer.html"), `{{ define "partials/footer" -}}
<footer><small>&copy; 2024 New Site</small></footer>
{{- end }}
`)

	// Demonstrate fallback (no define)
	writeIfMissing(filepath.Join(c.SrcDir, "pages", "index.html"), `<h1>{{ .Page.title }}</h1>
<p>{{ .Page.description }}</p>
`)
	writeIfMissing(filepath.Join(c.SrcDir, "pages", "index.json"), `{
  "title": "Home",
  "description": "Welcome to the homepage."
}
`)
	writeIfMissing(filepath.Join(c.SrcDir, "static", "css", "style.css"), `body{font-family:system-ui,Segoe UI,Roboto,Arial,sans-serif;margin:2rem}nav a{margin-right:1rem}`)

	return nil
}

func mustMkdir(p string) { _ = os.MkdirAll(p, 0o755) }

func writeIfMissing(path, content string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(content), 0o644)
}
