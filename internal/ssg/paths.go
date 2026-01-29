package ssg

import (
	"fmt"
	"path/filepath"
	"strings"
)

func pageConfigPathFromRel(rel string) (string, error) {
	if strings.HasSuffix(rel, ".tmpl") {
		return strings.TrimSuffix(rel, ".tmpl") + ".json", nil
	}
	if strings.HasSuffix(rel, ".html") {
		return strings.TrimSuffix(rel, ".html") + ".json", nil
	}
	return "", fmt.Errorf("page must end with .tmpl or .html")
}

func outPathFromRel(out, rel string, page map[string]any) (string, error) {
	var relBase string
	switch {
	case strings.HasSuffix(rel, ".tmpl"):
		relBase = strings.TrimSuffix(rel, ".tmpl")
	case strings.HasSuffix(rel, ".html"):
		relBase = strings.TrimSuffix(rel, ".html")
	default:
		return "", fmt.Errorf("page must end with .tmpl or .html: %s", rel)
	}

	ext := filepath.Ext(relBase) // ".xml" or ""
	stem := relBase
	outExt := ".html"
	if ext != "" {
		outExt = ext
		stem = strings.TrimSuffix(relBase, ext)
	}

	if outExt != ".html" {
		return filepath.Join(out, filepath.FromSlash(stem+outExt)), nil
	}

	if v, ok := page["slug"]; ok {
		s, _ := v.(string)
		s = strings.Trim(s, "/")
		if s == "" {
			return filepath.Join(out, "index.html"), nil
		}
		return filepath.Join(out, filepath.FromSlash(s), "index.html"), nil
	}

	stem = filepath.ToSlash(stem)
	if stem == "index" {
		return filepath.Join(out, "index.html"), nil
	}
	if strings.HasSuffix(stem, "/index") {
		dir := strings.TrimSuffix(stem, "/index")
		return filepath.Join(out, filepath.FromSlash(dir), "index.html"), nil
	}
	return filepath.Join(out, filepath.FromSlash(stem), "index.html"), nil
}
