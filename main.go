package main

import (
	"fmt"
	"os"

	"github.com/summonhim/gzgspg/internal/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
