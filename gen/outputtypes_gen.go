//go:build ignore

// outputtypes_gen prints the output type table consumed by README.md and
// docs/src/content/docs/configuration/output-types.mdx. Run via task generate.
package main

import (
	"fmt"
	"os"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
)

func main() {
	if err := printTable(); err != nil {
		fmt.Fprintf(os.Stderr, "outputtypes: %v\n", err)
		os.Exit(1)
	}
}

// printTable writes the table byte-stably: rows come from
// installable.OutputTypeTable in DocOrder, and Deploys/Activation print
// as-is (Deploys already holds its markdown link).
func printTable() error {
	out := os.Stdout

	if _, err := fmt.Fprintln(out, "| Type | Deploys | Activation |"); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(out, "|------|---------|------------|"); err != nil {
		return err
	}

	for _, row := range installable.OutputTypeTable() {
		line := fmt.Sprintf("| `%s` | %s | %s |", row.Type, row.Deploys, row.Activation)
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}

	return nil
}
