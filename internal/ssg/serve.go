package ssg

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

func Serve(c ServeConfig) error {
	devPrefix := "/_ssg/"

	state := &devState{}
	if err := Build(c.Config); err != nil {
		// Keep serving in dev mode even if build fails so the browser can show the error.
		fmt.Fprintln(os.Stderr, "SSG - Error:", humanizeError(err))
		state.setErr(err)
	}

	hub := newReloadHub()

	fmt.Fprintln(os.Stderr, "SSG - Serving: ", c.Out, "at http://localhost"+c.Addr)
	if c.Watch {
		fmt.Fprintln(os.Stderr, "SSG - Watching for changes")
		ctx := context.Background()
		go watchAndRebuild(ctx, c.Config, c.Interval, func(err error) {
			state.setErr(err)
			// Reload on both success and failure so the browser reflects the latest state.
			hub.Notify()
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc(joinURLPrefix(devPrefix, "events"), hub.Events)
	mux.HandleFunc(joinURLPrefix(devPrefix, "reload.js"), newReloadJSHandler(devPrefix))
	mux.Handle("/", newDevFileServerWithDevPrefix(c.Out, state, devPrefix))

	srv := &http.Server{
		Addr:              c.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}
