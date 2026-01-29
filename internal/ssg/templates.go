package ssg

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ---------------- hardening: allowed defines + template set guard ----------------

var defineRe = regexp.MustCompile(`(?s){{\s*define\s+"([^"]+)"`)

func validatePageDefines(src string, rel string) error {
	allowed := map[string]struct{}{
		"content":  {},
		"__page__": {},
	}
	matches := defineRe.FindAllStringSubmatch(src, -1)
	for _, m := range matches {
		name := m[1]
		if _, ok := allowed[name]; !ok {
			return fmt.Errorf("%s: page may not define template %q (allowed: content, __page__)", rel, name)
		}
	}
	return nil
}

func templateNameSet(t *template.Template) map[string]struct{} {
	out := map[string]struct{}{}
	for _, tt := range t.Templates() {
		out[tt.Name()] = struct{}{}
	}
	return out
}

func validateTemplateSet(t *template.Template, baseNames map[string]struct{}, rel string) error {
	allowedExtras := map[string]struct{}{
		"content":  {},
		"__page__": {},
	}
	for _, tt := range t.Templates() {
		name := tt.Name()
		if _, ok := baseNames[name]; ok {
			continue
		}
		if _, ok := allowedExtras[name]; ok {
			continue
		}
		return fmt.Errorf("%s: page introduced forbidden template %q", rel, name)
	}
	return nil
}

// ---------------- template base ----------------

func parseBase(src string, site map[string]any) (*template.Template, error) {
	date := func(layout string, args ...any) string {
		layout = strings.TrimSpace(layout)
		if layout == "" {
			layout = time.RFC3339
		}

		var v any
		if len(args) == 0 {
			v = time.Now().UTC()
		} else {
			v = args[0]
		}

		switch tv := v.(type) {
		case time.Time:
			return tv.UTC().Format(layout)
		case *time.Time:
			if tv == nil {
				return ""
			}
			return tv.UTC().Format(layout)
		case string:
			s := strings.TrimSpace(tv)
			if s == "" {
				return ""
			}
			for _, f := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02", "2006-01-02 15:04", "2006-01-02 15:04:05"} {
				if t, err := time.Parse(f, s); err == nil {
					return t.UTC().Format(layout)
				}
			}
			// If we couldn't parse it, just return the original string.
			return tv
		default:
			return fmt.Sprint(v)
		}
	}

	jsonFn := func(v any) template.JS {
		b, err := json.Marshal(v)
		if err != nil {
			return template.JS("null")
		}
		return template.JS(string(b))
	}

	absURL := func(p string) string {
		if p == "" {
			return ""
		}
		if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
			return p
		}
		base, _ := site["baseURL"].(string)
		base = strings.TrimRight(strings.TrimSpace(base), "/")
		if base == "" {
			return p
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		return base + p
	}

	t := template.New("ssg").Funcs(template.FuncMap{
		"safeHTML": func(v any) template.HTML { s, _ := v.(string); return template.HTML(s) },
		"absURL":   absURL,
		"date":     date,
		"json":     jsonFn,
	})

	for _, p := range mustFiles(filepath.Join(src, "layouts"), ".tmpl", ".html") {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		if _, err := t.Parse(string(b)); err != nil {
			return nil, fmt.Errorf("layout %s: %w", p, err)
		}
	}
	for _, p := range mustFiles(filepath.Join(src, "partials"), ".tmpl", ".html") {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		if _, err := t.Parse(string(b)); err != nil {
			return nil, fmt.Errorf("partial %s: %w", p, err)
		}
	}
	return t, nil
}

func mustFiles(root string, exts ...string) []string {
	files, _ := findFilesMulti(root, exts)
	sort.Strings(files)
	return files
}
