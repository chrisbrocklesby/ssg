package cli

import (
	"flag"
	"runtime"
	"time"

	"github.com/chrisbrocklesby/ssg/internal/ssg"
)

func buildCmd(args []string) error {
	c := parseBuildFlags("build", args)
	return ssg.Build(c)
}

func parseBuildFlags(name string, args []string) ssg.Config {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	src := fs.String("src", "./src", "src dir")
	out := fs.String("out", "./dist", "output dir")
	cache := fs.String("cache", "./.cache", "fetch cache dir")
	nocache := fs.Bool("nocache", false, "disable fetch cache")
	clean := fs.Bool("clean", false, "clean output dir")
	fetchTTL := fs.Duration("fetch-ttl", 1*time.Hour, "fetch cache TTL (e.g. 0s, 10m, 1h)")
	jobs := fs.Int("jobs", runtime.NumCPU(), "number of parallel page workers")
	_ = fs.Parse(args)

	j := *jobs
	if j < 1 {
		j = 1
	}

	return ssg.Config{
		Src:      *src,
		Out:      *out,
		Cache:    *cache,
		NoCache:  *nocache,
		Clean:    *clean,
		FetchTTL: *fetchTTL,
		Jobs:     j,
	}
}
