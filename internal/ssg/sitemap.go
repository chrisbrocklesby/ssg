package ssg

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type sitemapEntry struct {
	Loc     string
	LastMod time.Time
}

func fileToURL(baseURL, outDir, outFile string) string {
	rel, err := filepath.Rel(outDir, outFile)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)

	if rel == "index.html" {
		return baseURL + "/"
	}
	if strings.HasSuffix(rel, "/index.html") {
		rel = strings.TrimSuffix(rel, "index.html")
		return baseURL + "/" + rel
	}
	return baseURL + "/" + rel
}

func writeSitemap(path string, entries []sitemapEntry) error {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, e := range entries {
		b.WriteString("  <url><loc>")
		b.WriteString(xmlEscape(e.Loc))
		b.WriteString("</loc>")
		if !e.LastMod.IsZero() {
			b.WriteString("<lastmod>")
			b.WriteString(xmlEscape(e.LastMod.Format(time.RFC3339)))
			b.WriteString("</lastmod>")
		}
		b.WriteString("</url>\n")
	}
	b.WriteString("</urlset>\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
