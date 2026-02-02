package ssg

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func expectedOutFiles(c Config, pagesRoot string, pageFiles []string, baseURL string) (map[string]struct{}, error) {
	expected := map[string]struct{}{}
	if c.ExtractInlineStyles {
		outRel := strings.TrimSpace(c.ExtractInlineStylesOut)
		if outRel == "" {
			outRel = "css/inline.css"
		}
		expected[filepath.Clean(filepath.Join(c.Out, filepath.FromSlash(outRel)))] = struct{}{}
	}

	staticDir := filepath.Join(c.Src, "static")
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		err := filepath.WalkDir(staticDir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(staticDir, p)
			if err != nil {
				return err
			}
			expected[filepath.Clean(filepath.Join(c.Out, rel))] = struct{}{}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	anySitemapEntry := false
	for _, pagePath := range pageFiles {
		relOS, _ := filepath.Rel(pagesRoot, pagePath)
		rel := filepath.ToSlash(relOS)

		cfgRel, err := pageConfigPathFromRel(rel)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}

		page, err := loadObj(filepath.Join(pagesRoot, cfgRel))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", cfgRel, err)
		}
		if page == nil {
			page = map[string]any{}
		}
		if err := validatePage(page, rel); err != nil {
			return nil, err
		}
		if b, ok := page["draft"].(bool); ok && b {
			continue
		}

		outFile, err := outPathFromRel(c.Out, rel, page)
		if err != nil {
			return nil, err
		}
		outFile = filepath.Clean(outFile)
		expected[outFile] = struct{}{}

		if baseURL != "" && strings.HasSuffix(outFile, ".html") {
			if v, ok := page["sitemap"].(bool); ok && v == false {
				continue
			}
			anySitemapEntry = true
		}
	}

	if baseURL != "" && anySitemapEntry {
		expected[filepath.Clean(filepath.Join(c.Out, "sitemap.xml"))] = struct{}{}
	}

	return expected, nil
}

func pruneOutDir(outDir string, expected map[string]struct{}) error {
	outDir = filepath.Clean(outDir)

	var dirs []string
	err := filepath.WalkDir(outDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, p)
			return nil
		}

		p = filepath.Clean(p)
		if _, ok := expected[p]; ok {
			return nil
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		if filepath.Clean(dir) == outDir {
			continue
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		if len(ents) == 0 {
			_ = os.Remove(dir)
		}
	}
	return nil
}
