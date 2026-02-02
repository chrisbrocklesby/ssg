package ssg

import (
	"path/filepath"
	"runtime"
	"time"
)

// Options configures SSG when used as a library.
//
// Most fields are optional; sane defaults are applied.
// Paths are treated as relative to Root unless already absolute.
//
// Attach defaults to watch+reload enabled.
// Build defaults to watch disabled.
type Options struct {
	Root string

	Src   string
	Out   string
	Cache string

	NoCache  bool
	Clean    bool
	FetchTTL time.Duration
	Jobs     int

	// Watch enables rebuild-on-change.
	// If nil, defaults to true for Attach.
	Watch *bool

	// Interval controls how often the watcher scans for changes.
	// If zero, defaults to 500ms.
	Interval time.Duration

	// SitePrefix mounts the generated site under this URL path prefix.
	// Defaults to "/".
	SitePrefix string

	// DevPrefix mounts dev endpoints (SSE + reload script) under this URL path prefix.
	// Defaults to "/_ssg/".
	DevPrefix string
}

func (o Options) withDefaultsForAttach() Options {
	out := o
	if out.Root == "" {
		out.Root = "."
	}
	if out.Src == "" {
		out.Src = filepath.Join(out.Root, "src")
	} else if !filepath.IsAbs(out.Src) {
		out.Src = filepath.Join(out.Root, out.Src)
	}
	if out.Out == "" {
		out.Out = filepath.Join(out.Root, "dist")
	} else if !filepath.IsAbs(out.Out) {
		out.Out = filepath.Join(out.Root, out.Out)
	}
	if out.Cache == "" {
		out.Cache = filepath.Join(out.Root, ".cache")
	} else if !filepath.IsAbs(out.Cache) {
		out.Cache = filepath.Join(out.Root, out.Cache)
	}
	if out.FetchTTL == 0 {
		out.FetchTTL = 1 * time.Hour
	}
	if out.Jobs < 1 {
		out.Jobs = runtime.NumCPU()
		if out.Jobs < 1 {
			out.Jobs = 1
		}
	}
	if out.Interval == 0 {
		out.Interval = 500 * time.Millisecond
	}
	if out.SitePrefix == "" {
		out.SitePrefix = "/"
	}
	if out.DevPrefix == "" {
		out.DevPrefix = "/_ssg/"
	}
	return out
}

func (o Options) watchForAttach() bool {
	if o.Watch == nil {
		return true
	}
	return *o.Watch
}
