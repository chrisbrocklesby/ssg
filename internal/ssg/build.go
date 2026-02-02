package ssg

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

func Build(c Config) error {
	if c.Jobs < 1 {
		c.Jobs = runtime.NumCPU()
		if c.Jobs < 1 {
			c.Jobs = 1
		}
	}
	if _, err := os.Stat(c.Src); err != nil {
		return fmt.Errorf("src not found: %s", c.Src)
	}
	if c.Clean {
		_ = os.RemoveAll(c.Out)
	}
	if err := os.MkdirAll(c.Out, 0o755); err != nil {
		return err
	}

	site, err := loadObj(filepath.Join(c.Src, "site.json"))
	if err != nil {
		return fmt.Errorf("site.json: %w", err)
	}
	if site == nil {
		site = map[string]any{}
	}
	normalizeSite(site)
	if err := validateSite(site); err != nil {
		return err
	}

	data := map[string]any{}
	if err := loadData(filepath.Join(c.Src, "data"), data); err != nil {
		return err
	}

	fc := newFetch(c.Cache, c.NoCache, c.FetchTTL)
	if err := applyFetch(site, fc, "site"); err != nil {
		return err
	}
	normalizeSite(site)

	baseURL, _ := site["baseURL"].(string)
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")

	pagesRoot := filepath.Join(c.Src, "pages")
	pageFiles, err := findFilesMulti(pagesRoot, []string{".tmpl", ".html"})
	if err != nil {
		return err
	}
	sort.Strings(pageFiles)

	expected, err := expectedOutFiles(c, pagesRoot, pageFiles, baseURL)
	if err != nil {
		return err
	}

	if err := copyDir(filepath.Join(c.Src, "static"), c.Out); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	base, err := parseBase(c.Src, site)
	if err != nil {
		return err
	}
	if base.Lookup("default") == nil {
		return fmt.Errorf(`missing layouts/default.html with {{ define "default" }} (also supports default.tmpl)`)
	}
	baseNames := templateNameSet(base)

	type pageJob struct {
		pagePath string
		rel      string
	}

	jobs := make(chan pageJob)
	stop := make(chan struct{})
	var stopOnce sync.Once

	var wg sync.WaitGroup
	var hadErr atomic.Bool
	var firstErr error
	var firstErrOnce sync.Once

	var smMu sync.Mutex
	var sitemapEntries []sitemapEntry

	worker := func() {
		defer wg.Done()
		for job := range jobs {
			if hadErr.Load() {
				continue
			}
			if err := buildOnePage(base, baseNames, fc, site, data, baseURL, pagesRoot, c.Out, job.pagePath, job.rel, &sitemapEntries, &smMu); err != nil {
				firstErrOnce.Do(func() { firstErr = err })
				hadErr.Store(true)
				stopOnce.Do(func() { close(stop) })
			}
		}
	}

	for i := 0; i < c.Jobs; i++ {
		wg.Add(1)
		go worker()
	}

sendLoop:
	for _, pagePath := range pageFiles {
		relOS, _ := filepath.Rel(pagesRoot, pagePath)
		rel := filepath.ToSlash(relOS)
		select {
		case <-stop:
			break sendLoop
		case jobs <- pageJob{pagePath: pagePath, rel: rel}:
		}
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}

	if baseURL != "" && len(sitemapEntries) > 0 {
		sort.Slice(sitemapEntries, func(i, j int) bool { return sitemapEntries[i].Loc < sitemapEntries[j].Loc })
		if err := writeSitemap(filepath.Join(c.Out, "sitemap.xml"), sitemapEntries); err != nil {
			return err
		}
	}

	if err := pruneOutDir(c.Out, expected); err != nil {
		return err
	}
	return nil
}

func buildOnePage(
	base *template.Template,
	baseNames map[string]struct{},
	fc *fetcher,
	site map[string]any,
	data map[string]any,
	baseURL string,
	pagesRoot string,
	outDir string,
	pagePath string,
	rel string,
	sitemapEntries *[]sitemapEntry,
	smMu *sync.Mutex,
) error {
	cfgRel, err := pageConfigPathFromRel(rel)
	if err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	page, err := loadObj(filepath.Join(pagesRoot, cfgRel))
	if err != nil {
		return fmt.Errorf("%s: %w", cfgRel, err)
	}
	if page == nil {
		page = map[string]any{}
	}
	if err := validatePage(page, rel); err != nil {
		return err
	}
	if b, ok := page["draft"].(bool); ok && b {
		return nil
	}
	if err := applyFetch(page, fc, rel); err != nil {
		return err
	}

	layout := resolveLayout(page)
	if layout != "none" && base.Lookup(layout) == nil {
		return fmt.Errorf("%s: layout %q not found", rel, layout)
	}

	outFile, err := outPathFromRel(outDir, rel, page)
	if err != nil {
		return err
	}

	info, err := os.Stat(pagePath)
	if err != nil {
		return err
	}
	pageMTime := info.ModTime().UTC()

	bt, err := os.ReadFile(pagePath)
	if err != nil {
		return err
	}
	src := string(bt)

	if err := validatePageDefines(src, rel); err != nil {
		return err
	}

	t, err := base.Clone()
	if err != nil {
		return err
	}

	if _, err := t.Parse(src); err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}

	if layout == "none" {
		if t.Lookup("__page__") == nil {
			wrapped := "{{ define \"__page__\" -}}\n" + src + "\n{{- end }}\n"
			if _, err := t.Parse(wrapped); err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
		}
	} else {
		if t.Lookup("content") == nil {
			wrapped := "{{ define \"content\" -}}\n" + src + "\n{{- end }}\n"
			if _, err := t.Parse(wrapped); err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
		}
	}

	if err := validateTemplateSet(t, baseNames, rel); err != nil {
		return err
	}

	ctx := map[string]any{"Site": site, "Page": page, "Data": data}
	var buf bytes.Buffer
	if layout == "none" {
		if err := t.ExecuteTemplate(&buf, "__page__", ctx); err != nil {
			return fmt.Errorf("%s: render: %w", rel, err)
		}
	} else {
		if err := t.ExecuteTemplate(&buf, layout, ctx); err != nil {
			return fmt.Errorf("%s: render: %w", rel, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(outFile), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outFile, buf.Bytes(), 0o644); err != nil {
		return err
	}

	if baseURL != "" && strings.HasSuffix(outFile, ".html") {
		if v, ok := page["sitemap"].(bool); ok && v == false {
			return nil
		}
		loc := fileToURL(baseURL, outDir, outFile)
		if loc != "" {
			smMu.Lock()
			*sitemapEntries = append(*sitemapEntries, sitemapEntry{Loc: loc, LastMod: pageMTime})
			smMu.Unlock()
		}
	}

	return nil
}

func resolveLayout(page map[string]any) string {
	layout := "default"
	if s, ok := page["layout"].(string); ok {
		s = strings.TrimSpace(strings.TrimSuffix(s, ".tmpl"))
		if strings.EqualFold(s, "none") {
			return "none"
		}
		if s != "" {
			layout = s
		}
	}
	return layout
}
