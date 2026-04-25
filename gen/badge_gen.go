//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/mihakrumpestar/panix/gen"
)

func main() {
	if err := gen.LocBadge(); err != nil {
		fmt.Fprintf(os.Stderr, "badge: %v\n", err)
		os.Exit(1)
	}
}
