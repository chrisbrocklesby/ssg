package ssg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

func loadObj(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var v any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected top-level object")
	}
	return m, nil
}

func validateSite(m map[string]any) error {
	if v, ok := m["baseURL"]; ok && v != nil {
		if _, ok := v.(string); !ok {
			return fmt.Errorf(`site: "baseURL" must be string`)
		}
	}
	if v, ok := m["fetch"]; ok && v != nil {
		if _, ok := v.(map[string]any); !ok {
			return fmt.Errorf(`site: "fetch" must be object`)
		}
	}
	return nil
}

func normalizeSite(m map[string]any) {
	if m == nil {
		return
	}
	// Accept baseURL in any casing, plus common separators.
	if _, ok := m["baseURL"]; !ok {
		for k, v := range m {
			kl := strings.ToLower(k)
			switch kl {
			case "baseurl", "base_url", "base-url":
				m["baseURL"] = v
				delete(m, k)
				return
			}
		}
	}
}

func validatePage(m map[string]any, label string) error {
	typeCheck := func(k string, want string, ok bool) error {
		if !ok {
			return fmt.Errorf("%s: %q must be %s", label, k, want)
		}
		return nil
	}
	if v, ok := m["title"]; ok && v != nil {
		if err := typeCheck("title", "string", isStr(v)); err != nil {
			return err
		}
	}
	if v, ok := m["layout"]; ok && v != nil {
		if err := typeCheck("layout", "string", isStr(v)); err != nil {
			return err
		}
	}
	if v, ok := m["draft"]; ok && v != nil {
		if err := typeCheck("draft", "bool", isBool(v)); err != nil {
			return err
		}
	}
	if v, ok := m["slug"]; ok && v != nil {
		if err := typeCheck("slug", "string", isStr(v)); err != nil {
			return err
		}
	}
	if v, ok := m["fetch"]; ok && v != nil {
		if err := typeCheck("fetch", "object", isObj(v)); err != nil {
			return err
		}
	}
	if v, ok := m["sitemap"]; ok && v != nil {
		if err := typeCheck("sitemap", "bool", isBool(v)); err != nil {
			return err
		}
	}
	return nil
}

func isStr(v any) bool  { _, ok := v.(string); return ok }
func isBool(v any) bool { _, ok := v.(bool); return ok }
func isObj(v any) bool  { _, ok := v.(map[string]any); return ok }
