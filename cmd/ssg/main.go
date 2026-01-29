package main

import (
	"os"

	"github.com/chrisbrocklesby/ssg/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args))
}
