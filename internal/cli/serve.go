package cli

import (
	"flag"
	"runtime"
	"time"

	"github.com/chrisbrocklesby/ssg/internal/ssg"
)

func serveCmd(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)

	src := fs.String("src", "./src", "src dir")
	out := fs.String("out", "./dist", "output dir")
	cache := fs.String("cache", "./.cache", "fetch cache dir")
	nocache := fs.Bool("nocache", false, "disable fetch cache")
	clean := fs.Bool("clean", false, "clean output dir")
	fetchTTL := fs.Duration("fetch-ttl", 1*time.Hour, "fetch cache TTL")
	jobs := fs.Int("jobs", runtime.NumCPU(), "parallel workers")

	addr := fs.String("addr", ":8080", "listen address")
	watch := fs.Bool("watch", false, "watch src and rebuild")
	interval := fs.Duration("interval", 500*time.Millisecond, "watch interval")

	_ = fs.Parse(args)

	c := ssg.Config{
		Src:      *src,
		Out:      *out,
		Cache:    *cache,
		NoCache:  *nocache,
		Clean:    *clean,
		FetchTTL: *fetchTTL,
		Jobs:     max(1, *jobs),
	}

	return ssg.Serve(ssg.ServeConfig{Config: c, Addr: *addr, Watch: *watch, Interval: *interval})
}
