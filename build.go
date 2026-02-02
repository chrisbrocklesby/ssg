package ssg

import (
	"context"

	impl "github.com/chrisbrocklesby/ssg/internal/ssg"
)

// Build runs a single site build.
func Build(ctx context.Context, opts Options) error {
	_ = ctx
	opts = opts.withDefaultsForAttach()

	cfg := impl.Config{
		Src:      opts.Src,
		Out:      opts.Out,
		Cache:    opts.Cache,
		NoCache:  opts.NoCache,
		Clean:    opts.Clean,
		FetchTTL: opts.FetchTTL,
		Jobs:     opts.Jobs,

		ExtractInlineStyles:    opts.ExtractInlineStyles,
		ExtractInlineStylesOut: opts.ExtractInlineStylesOut,
	}
	return impl.Build(cfg)
}
