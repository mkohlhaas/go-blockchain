package main

import (
	"github.com/mkohlhaas/golang-blockchain/cli"
	"os"
)

func main() {
	defer os.Exit(0)
	cmd := cli.CommandLine{}
	cmd.Run()
}
