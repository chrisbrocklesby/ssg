package ssg

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

// Attach mounts SSG dev handlers into an existing mux.
//
// If mux is nil, http.DefaultServeMux is used.
//
// sitePrefix defaults to "/".
// devPrefix defaults to "/_ssg/".
//
// The returned stop function cancels background watchers started by Attach.
func Attach(ctx context.Context, mux *http.ServeMux, c ServeConfig, sitePrefix, devPrefix string) (stop func(), err error) {
	if mux == nil {
		mux = http.DefaultServeMux
	}
	if sitePrefix == "" {
		sitePrefix = "/"
	}
	if devPrefix == "" {
		devPrefix = "/_ssg/"
	}

	sitePrefix = normalizeURLPrefix(sitePrefix)
	devPrefix = normalizeURLPrefix(devPrefix)

	ctxWatch, cancel := context.WithCancel(ctx)

	state := &devState{}
	if err := Build(c.Config); err != nil {
		// Keep serving in dev mode even if build fails so the browser can show the error.
		fmt.Fprintln(os.Stderr, "SSG - Error:", humanizeError(err))
		state.setErr(err)
	}

	hub := newReloadHub()

	eventsPath := joinURLPrefix(devPrefix, "events")
	reloadJSPath := joinURLPrefix(devPrefix, "reload.js")

	mux.HandleFunc(eventsPath, hub.Events)
	mux.HandleFunc(reloadJSPath, newReloadJSHandler(devPrefix))

	siteHandler := newDevFileServerWithDevPrefix(c.Out, state, devPrefix)
	if sitePrefix == "/" {
		mux.Handle("/", siteHandler)
	} else {
		mux.Handle(sitePrefix, http.StripPrefix(strings.TrimSuffix(sitePrefix, "/"), siteHandler))
	}

	if c.Watch {
		fmt.Fprintln(os.Stderr, "SSG - Watching for changes")
		go watchAndRebuild(ctxWatch, c.Config, c.Interval, func(err error) {
			state.setErr(err)
			// Reload on both success and failure so the browser reflects the latest state.
			hub.Notify()
		})
	}

	var once sync.Once
	return func() {
		once.Do(cancel)
	}, nil
}
