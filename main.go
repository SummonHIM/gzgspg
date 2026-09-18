package main

import (
	"fmt"

	"github.com/summonhim/gzgspg/internal/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		fmt.Println(err)
	}
}
