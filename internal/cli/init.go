package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ssg/internal/ssg"
)

func initCmd(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: ssg init <projectDir>")
	}
	root := fs.Arg(0)
	src := filepath.Join(root, "src")

	cfg := ssg.InitConfig{SrcDir: src}
	if err := ssg.Init(cfg); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "SSG - Initialised", root)
	return nil
}
