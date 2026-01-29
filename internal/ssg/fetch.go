package ssg

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ---------------- fetch (TTL + revalidation + in-flight de-dupe) ----------------

type fetcher struct {
	nocache bool
	dir     string
	ttl     time.Duration

	mu       sync.Mutex
	mem      map[string]memEntry
	inflight map[string]*inflightCall
	hc       *http.Client
}

type inflightCall struct {
	wg  sync.WaitGroup
	val any
	err error
}

type memEntry struct {
	val     any
	body    []byte
	meta    cacheMeta
	fetched time.Time
}

type cacheMeta struct {
	ContentType  string `json:"contentType"`
	As           string `json:"as"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	FetchedUnix  int64  `json:"fetchedUnix"`
	SourceURL    string `json:"sourceURL"`
}

func newFetch(cacheDir string, nocache bool, ttl time.Duration) *fetcher {
	return &fetcher{
		nocache:  nocache,
		dir:      cacheDir,
		ttl:      ttl,
		mem:      map[string]memEntry{},
		inflight: map[string]*inflightCall{},
		hc:       &http.Client{Timeout: 25 * time.Second},
	}
}

func applyFetch(obj map[string]any, f *fetcher, label string) error {
	raw, ok := obj["fetch"]
	if !ok {
		return nil
	}
	reqs, err := parseFetch(raw)
	if err != nil {
		return fmt.Errorf("%s: fetch: %w", label, err)
	}

	keys := make([]string, 0, len(reqs))
	for k := range reqs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := map[string]any{}
	for _, k := range keys {
		v, err := f.get(reqs[k])
		if err != nil {
			return fmt.Errorf("%s: fetch[%s]: %w", label, k, err)
		}
		out[k] = v
	}
	obj["fetch"] = out
	return nil
}

func parseFetch(v any) (map[string]FetchReq, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("must be object")
	}
	out := map[string]FetchReq{}
	for k, vv := range m {
		switch x := vv.(type) {
		case string:
			out[k] = FetchReq{URL: x, As: "auto"}
		case map[string]any:
			u, _ := x["url"].(string)
			if strings.TrimSpace(u) == "" {
				return nil, fmt.Errorf("%s: missing url", k)
			}
			as := "auto"
			if s, ok := x["as"].(string); ok && s != "" {
				as = strings.ToLower(strings.TrimSpace(s))
			}
			if as != "auto" && as != "json" && as != "text" {
				return nil, fmt.Errorf("%s: as must be auto|json|text", k)
			}
			out[k] = FetchReq{URL: u, As: as}
		default:
			return nil, fmt.Errorf("%s: must be url string or {url,as}", k)
		}
	}
	return out, nil
}

func (f *fetcher) get(r FetchReq) (any, error) {
	url := strings.TrimSpace(r.URL)
	as := r.As
	if as == "" {
		as = "auto"
	}
	key := hashString(as + "\n" + url)

	if f.nocache {
		body, meta, err := f.fetchWithRevalidate(url, as, cacheMeta{}, nil)
		if err != nil {
			return nil, err
		}
		return decode(body, meta.ContentType, meta.As)
	}

	// Memory cache hit
	f.mu.Lock()
	if e, ok := f.mem[key]; ok {
		if f.ttl <= 0 || time.Since(e.fetched) <= f.ttl {
			v := e.val
			f.mu.Unlock()
			return v, nil
		}
	}
	// In-flight de-dupe
	if c, ok := f.inflight[key]; ok {
		f.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	call := &inflightCall{}
	call.wg.Add(1)
	f.inflight[key] = call
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.inflight, key)
		f.mu.Unlock()
		call.wg.Done()
	}()

	// Disk cache
	if f.dir != "" {
		if val, body, meta, ok, err := f.readDiskDecoded(key); err == nil && ok {
			f.mu.Lock()
			f.mem[key] = memEntry{val: val, body: body, meta: meta, fetched: time.Unix(meta.FetchedUnix, 0)}
			f.mu.Unlock()

			// TTL revalidate if stale
			if f.ttl > 0 && time.Since(time.Unix(meta.FetchedUnix, 0)) > f.ttl {
				body2, meta2, err := f.fetchWithRevalidate(url, as, meta, body)
				if err == nil {
					val2, err2 := decode(body2, meta2.ContentType, meta2.As)
					if err2 == nil {
						f.mu.Lock()
						f.mem[key] = memEntry{val: val2, body: body2, meta: meta2, fetched: time.Unix(meta2.FetchedUnix, 0)}
						f.mu.Unlock()
						_ = f.writeDisk(key, body2, meta2)
						call.val = val2
						return val2, nil
					}
				}
				// If revalidate fails, return stale cached value
			}
			call.val = val
			return val, nil
		}
	}

	// Fresh fetch
	body, meta, err := f.fetchWithRevalidate(url, as, cacheMeta{}, nil)
	if err != nil {
		call.err = err
		return nil, err
	}
	val, err := decode(body, meta.ContentType, meta.As)
	if err != nil {
		call.err = err
		return nil, err
	}

	f.mu.Lock()
	f.mem[key] = memEntry{val: val, body: body, meta: meta, fetched: time.Unix(meta.FetchedUnix, 0)}
	f.mu.Unlock()
	if f.dir != "" {
		_ = f.writeDisk(key, body, meta)
	}
	call.val = val
	return val, nil
}

func (f *fetcher) fetchWithRevalidate(url, as string, prior cacheMeta, priorBody []byte) ([]byte, cacheMeta, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "ssg/1.0")
	if prior.ETag != "" {
		req.Header.Set("If-None-Match", prior.ETag)
	}
	if prior.LastModified != "" {
		req.Header.Set("If-Modified-Since", prior.LastModified)
	}

	resp, err := f.hc.Do(req)
	if err != nil {
		return nil, cacheMeta{}, err
	}
	defer resp.Body.Close()

	now := time.Now().UTC()

	// Correct 304 handling: reuse priorBody
	if resp.StatusCode == http.StatusNotModified && (prior.ETag != "" || prior.LastModified != "") {
		if priorBody == nil {
			// Internal state issue: retry once without validators.
			return f.fetchWithRevalidate(url, as, cacheMeta{}, nil)
		}
		meta := prior
		meta.FetchedUnix = now.Unix()
		if et := resp.Header.Get("ETag"); et != "" {
			meta.ETag = et
		}
		if lm := resp.Header.Get("Last-Modified"); lm != "" {
			meta.LastModified = lm
		}
		meta.SourceURL = url
		if meta.As == "" {
			meta.As = as
		}
		return priorBody, meta, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, cacheMeta{}, fmt.Errorf("http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, cacheMeta{}, err
	}
	meta := cacheMeta{
		ContentType:  resp.Header.Get("Content-Type"),
		As:           as,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		FetchedUnix:  now.Unix(),
		SourceURL:    url,
	}
	return body, meta, nil
}

func decode(body []byte, ctype, as string) (any, error) {
	switch as {
	case "text":
		return string(body), nil
	case "json":
		var v any
		d := json.NewDecoder(bytes.NewReader(body))
		d.UseNumber()
		err := d.Decode(&v)
		return v, err
	default:
		ct := strings.ToLower(ctype)
		trim := bytes.TrimSpace(body)
		looksJSON := len(trim) > 0 && (trim[0] == '{' || trim[0] == '[')
		if strings.Contains(ct, "application/json") || strings.Contains(ct, "+json") || looksJSON {
			var v any
			d := json.NewDecoder(bytes.NewReader(body))
			d.UseNumber()
			err := d.Decode(&v)
			return v, err
		}
		return string(body), nil
	}
}

func (f *fetcher) readDiskDecoded(key string) (val any, body []byte, meta cacheMeta, ok bool, err error) {
	bin := filepath.Join(f.dir, key+".bin")
	metaPath := filepath.Join(f.dir, key+".meta.json")
	body, err = os.ReadFile(bin)
	if err != nil {
		return nil, nil, cacheMeta{}, false, err
	}
	mb, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, nil, cacheMeta{}, false, err
	}
	if err := json.Unmarshal(mb, &meta); err != nil {
		return nil, nil, cacheMeta{}, false, err
	}
	val, err = decode(body, meta.ContentType, meta.As)
	if err != nil {
		return nil, nil, cacheMeta{}, false, err
	}
	return val, body, meta, true, nil
}

func (f *fetcher) writeDisk(key string, body []byte, meta cacheMeta) error {
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(f.dir, key+".bin"), body, 0o644); err != nil {
		return err
	}
	mb, _ := json.Marshal(meta)
	return os.WriteFile(filepath.Join(f.dir, key+".meta.json"), mb, 0o644)
}

func hashString(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}
