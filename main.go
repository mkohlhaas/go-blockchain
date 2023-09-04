// Package main is the entry point to our program.
package main

import (
	"os"

	"github.com/mkohlhaas/gobc/cli"
)

func main() {
	defer os.Exit(0)
	cmd := new(cli.CommandLine)
	cmd.Run()
}
