package ssg

import (
	"context"
	"net/http"

	impl "github.com/chrisbrocklesby/ssg/internal/ssg"
)

// Attach mounts SSG's dev site + live reload endpoints into an existing mux.
//
// If mux is nil, http.DefaultServeMux is used.
//
// Typical use:
//
//	stop, err := ssg.Attach(ctx, nil, ssg.Options{Root: "."})
//	defer stop()
func Attach(ctx context.Context, mux *http.ServeMux, opts Options) (stop func(), err error) {
	opts = opts.withDefaultsForAttach()

	cfg := impl.ServeConfig{
		Config: impl.Config{
			Src:      opts.Src,
			Out:      opts.Out,
			Cache:    opts.Cache,
			NoCache:  opts.NoCache,
			Clean:    opts.Clean,
			FetchTTL: opts.FetchTTL,
			Jobs:     opts.Jobs,
		},
		Watch:    opts.watchForAttach(),
		Interval: opts.Interval,
	}

	return impl.Attach(ctx, mux, cfg, opts.SitePrefix, opts.DevPrefix)
}
