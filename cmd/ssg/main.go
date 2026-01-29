package main

import (
	"os"

	"ssg/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args))
}
