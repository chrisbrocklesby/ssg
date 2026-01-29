package ssg

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func watchAndRebuild(ctx context.Context, c Config, interval time.Duration, onRebuild func(err error)) {
	type fp struct {
		kind string // "text" or "stat"
		hash string
		mt   int64
		sz   int64
	}

	last := map[string]fp{}
	first := true
	staticRoot := filepath.ToSlash(filepath.Join(c.Src, "static")) + "/"

	srcClean := filepath.Clean(c.Src)
	outClean := filepath.Clean(c.Out)
	outSlash := filepath.ToSlash(outClean)
	outInsideSrc := strings.HasPrefix(outClean, srcClean+string(os.PathSeparator)) || outClean == srcClean

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		changed := false
		seen := map[string]struct{}{}

		_ = filepath.WalkDir(c.Src, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			// If the output folder is inside src, ignore generated files to prevent rebuild loops.
			if outInsideSrc {
				ps := filepath.ToSlash(filepath.Clean(p))
				if ps == outSlash || strings.HasPrefix(ps, outSlash+"/") {
					return nil
				}
			}

			name := d.Name()
			isText := strings.HasSuffix(name, ".tmpl") || strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".json")
			isStatic := strings.HasPrefix(filepath.ToSlash(p), staticRoot)
			if !isText && !isStatic {
				return nil
			}

			seen[p] = struct{}{}
			info, err := d.Info()
			if err != nil {
				return nil
			}

			var cur fp
			if isText {
				mt := info.ModTime().UnixNano()
				sz := info.Size()
				if prev, ok := last[p]; ok && prev.kind == "text" && prev.mt == mt && prev.sz == sz {
					cur = prev
				} else {
					b, err := os.ReadFile(p)
					if err != nil {
						return nil
					}
					cur = fp{kind: "text", mt: mt, sz: sz, hash: hashBytes(b)}
				}
			} else {
				cur = fp{kind: "stat", mt: info.ModTime().UnixNano(), sz: info.Size()}
			}

			prev, ok := last[p]
			if !ok {
				if !first {
					changed = true
				}
				last[p] = cur
				return nil
			}

			diff := prev.kind != cur.kind ||
				(prev.kind == "text" && prev.hash != cur.hash) ||
				(prev.kind == "stat" && (prev.mt != cur.mt || prev.sz != cur.sz))

			if diff {
				changed = true
				last[p] = cur
			}
			return nil
		})

		if !first {
			for p := range last {
				if _, ok := seen[p]; !ok {
					delete(last, p)
					changed = true
				}
			}
		}
		first = false

		if changed {
			fmt.Fprintln(os.Stderr, "SSG - Rebuilding…")
			if err := Build(c); err != nil {
				fmt.Fprintln(os.Stderr, "SSG - Error:", humanizeError(err))
				onRebuild(err)
			} else {
				fmt.Fprintln(os.Stderr, "SSG - Rebuild Complete ✓")
				onRebuild(nil)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
