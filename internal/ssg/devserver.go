package ssg

import (
	"bytes"
	"html"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

func normalizeURLPrefix(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

func joinURLPrefix(prefix, elem string) string {
	prefix = normalizeURLPrefix(prefix)
	elem = strings.TrimPrefix(elem, "/")
	return prefix + elem
}

// ---------------- Hot Reload (SSE) ----------------

type reloadHub struct {
	mu      sync.Mutex
	clients map[chan struct{}]struct{}
}

func newReloadHub() *reloadHub {
	return &reloadHub{clients: map[chan struct{}]struct{}{}}
}

func (h *reloadHub) Notify() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (h *reloadHub) Events(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Accel-Buffering", "no")

	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
		close(ch)
	}()

	_, _ = io.WriteString(w, ": connected\n\n")
	f.Flush()

	keep := time.NewTicker(15 * time.Second)
	defer keep.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keep.C:
			_, _ = io.WriteString(w, ": keepalive\n\n")
			f.Flush()
		case <-ch:
			_, _ = io.WriteString(w, "data: reload\n\n")
			f.Flush()
		}
	}
}

func newReloadJSHandler(devPrefix string) http.HandlerFunc {
	eventsPath := joinURLPrefix(devPrefix, "events")

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = io.WriteString(w, `(() => {
  try {
    const es = new EventSource("`+eventsPath+`");
    es.onmessage = () => location.reload();
  } catch (_) {}
})();`)
	}
}

// Dev HTML injection server (only in serve)

type devFileServer struct {
	root        http.Dir
	fs          http.Handler
	st          *devState
	devPrefix   string
	reloadJSURL string

	mu    sync.RWMutex
	cache map[string]cachedHTML
}

type cachedHTML struct {
	mt   int64
	size int64
	body []byte
}

func newDevFileServer(outDir string, st *devState) http.Handler {
	return newDevFileServerWithDevPrefix(outDir, st, "/_ssg/")
}

func newDevFileServerWithDevPrefix(outDir string, st *devState, devPrefix string) http.Handler {
	devPrefix = normalizeURLPrefix(devPrefix)
	return &devFileServer{
		root:        http.Dir(outDir),
		fs:          http.FileServer(http.Dir(outDir)),
		st:          st,
		devPrefix:   devPrefix,
		reloadJSURL: joinURLPrefix(devPrefix, "reload.js"),
		cache:       map[string]cachedHTML{},
	}
}

func (s *devFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, s.devPrefix) {
		http.NotFound(w, r)
		return
	}

	path := cleanURLPath(r.URL.Path)

	if ext := strings.ToLower(filepath.Ext(path)); ext != "" && ext != ".html" {
		// In dev mode, make asset changes show up reliably even when the browser
		// would otherwise serve them from its cache.
		w.Header().Set("Cache-Control", "no-cache")
		s.fs.ServeHTTP(w, r)
		return
	}

	// If the last build failed, show an in-browser error page for HTML routes.
	if s.st != nil {
		if err := s.st.getErr(); err != nil {
			w.Header().Set("Content-Type", contentTypeHTML())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(renderBuildErrorHTML(err, s.reloadJSURL))
			return
		}
	}

	filePath := path
	if strings.HasSuffix(filePath, "/") || filepath.Ext(filePath) == "" {
		filePath = filePath + "index.html"
	}
	if filepath.Ext(filePath) == "" {
		filePath += ".html"
	}

	f, err := s.root.Open(strings.TrimPrefix(filePath, "/"))
	if err != nil {
		s.fs.ServeHTTP(w, r)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		s.fs.ServeHTTP(w, r)
		return
	}
	mt := info.ModTime().UnixNano()
	sz := info.Size()

	// Allow fast back/forward and link navigation by letting the browser revalidate.
	modTime := info.ModTime().UTC()
	w.Header().Set("Last-Modified", modTime.Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "no-cache")
	if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := time.Parse(http.TimeFormat, ims); err == nil {
			// HTTP time resolution is seconds.
			if !modTime.Truncate(time.Second).After(t) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	s.mu.RLock()
	if c, ok := s.cache[filePath]; ok && c.mt == mt && c.size == sz {
		s.mu.RUnlock()
		w.Header().Set("Content-Type", contentTypeHTML())
		w.Header().Set("Content-Length", strconv.Itoa(len(c.body)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(c.body)
		return
	}
	s.mu.RUnlock()

	b, err := io.ReadAll(f)
	if err != nil {
		s.fs.ServeHTTP(w, r)
		return
	}

	out := injectReloadJS(b, s.reloadJSURL)
	s.mu.Lock()
	s.cache[filePath] = cachedHTML{mt: mt, size: sz, body: out}
	s.mu.Unlock()

	w.Header().Set("Content-Type", contentTypeHTML())
	w.Header().Set("Content-Length", strconv.Itoa(len(out)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

func contentTypeHTML() string {
	ct := mime.TypeByExtension(".html")
	if ct == "" {
		ct = "text/html; charset=utf-8"
	}
	return ct
}

func cleanURLPath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	trailing := strings.HasSuffix(p, "/")
	p = filepath.ToSlash(filepath.Clean(p))
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if trailing && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	if p == "/." {
		p = "/"
	}
	return p
}

func injectReloadJS(b []byte, reloadJSURL string) []byte {
	if reloadJSURL == "" {
		reloadJSURL = "/_ssg/reload.js"
	}
	snippet := []byte(`<script defer src="` + reloadJSURL + `"></script>`)
	lower := bytes.ToLower(b)

	if i := bytes.LastIndex(lower, []byte("</body>")); i >= 0 {
		var out bytes.Buffer
		out.Grow(len(b) + len(snippet) + 8)
		out.Write(b[:i])
		out.Write(snippet)
		out.Write(b[i:])
		return out.Bytes()
	}
	return append(b, snippet...)
}

func renderBuildErrorHTML(err error, reloadJSURL string) []byte {
	if reloadJSURL == "" {
		reloadJSURL = "/_ssg/reload.js"
	}
	msg := html.EscapeString(err.Error())
	out := `<!doctype html>
<html>
<head>
	<meta charset="utf-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<title>SSG Error</title>
	<style>
		#ssg-error {
			font-family: Arial, sans-serif;
			border: 2px solid #ff0000;
			padding: 20px;
			margin: 50px auto;
			max-width: 800px;
			background-color: #ffe6e6;
			border-radius: 15px;
			color: #cc0000;
		}
		#ssg-header {
			font-size: 24px;
			font-weight: bold;
			margin-bottom: 10px;
		}
		#ssg-message {
			font-size: 16px;
			white-space: pre-wrap;
			overflow-wrap: anywhere;
		}
		#ssg-hint {
			margin-top: 12px;
			font-size: 13px;
			color: #7a0000;
			opacity: 0.9;
		}
	</style>
</head>
<body>
	<div id="ssg-error">
		<div id="ssg-header">Static Generator Error</div>
		<div id="ssg-message">` + msg + `<!-- Error ` + msg + ` --></div>
		<div id="ssg-hint">Fix the error in your source files; this page will auto-reload on rebuild.</div>
	</div>
	<script defer src="` + reloadJSURL + `"></script>
</body>
</html>
`
	return []byte(out)
}
