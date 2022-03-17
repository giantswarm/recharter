package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/giantswarm/recharter/cmd/run"
)

func main() {
	// This is for the future if we decide to add more commands.
	if len(os.Args) < 2 || os.Args[1] != "run" {
		name := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stderr, "usage: %s run [FLAGS]\n", name)
		os.Exit(1)
	}

	// For the flag parsing in the command.
	os.Args = os.Args[1:]

	err, code := run.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
	}
	os.Exit(code)
}
