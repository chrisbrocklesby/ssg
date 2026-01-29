package ssg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func loadData(root string, into map[string]any) error {
	files, err := findFilesMulti(root, []string{".json"})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	sort.Strings(files)
	for _, p := range files {
		obj, err := loadObj(p)
		if err != nil {
			return fmt.Errorf("data %s: %w", p, err)
		}
		rel, _ := filepath.Rel(root, p)
		key := strings.TrimSuffix(filepath.ToSlash(rel), ".json")
		if err := setPath(into, key, obj); err != nil {
			return fmt.Errorf("data %s: %w", key, err)
		}
	}
	return nil
}

func setPath(root map[string]any, path string, val any) error {
	parts := strings.Split(path, "/")
	cur := root
	for i, k := range parts {
		if i == len(parts)-1 {
			if _, exists := cur[k]; exists {
				return fmt.Errorf("collision")
			}
			cur[k] = val
			return nil
		}
		if next, ok := cur[k]; ok {
			m, ok := next.(map[string]any)
			if !ok {
				return fmt.Errorf("collision")
			}
			cur = m
			continue
		}
		m := map[string]any{}
		cur[k] = m
		cur = m
	}
	return nil
}
