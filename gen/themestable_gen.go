//go:build ignore

// themestable_gen prints the built-in themes table consumed by
// docs/src/content/docs/configuration/themes.mdx. Run via task generate.
package main

import (
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/flags"
	"github.com/mihakrumpestar/panix/internal/config/theme"
)

func main() {
	if err := printTable(); err != nil {
		fmt.Fprintf(os.Stderr, "themestable: %v\n", err)
		os.Exit(1)
	}
}

// printTable writes the table byte-stably: panix first as the default, then
// the remaining themes sorted alphabetically, each row showing its mode
// (dark or light) exactly as the theme declares it.
func printTable() error {
	out := os.Stdout

	if _, err := fmt.Fprintln(out, "| Name | Terminal background |"); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(out, "|------|---------------------|"); err != nil {
		return err
	}

	modes := theme.BuiltinThemeModes()

	defaultName, err := defaultThemeName()
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(out, "| `%s` | %s (default) |\n", defaultName, modes[defaultName]); err != nil {
		return err
	}

	names := slices.DeleteFunc(slices.Sorted(maps.Keys(modes)), func(name string) bool {
		return name == defaultName
	})

	for _, name := range names {
		if _, err := fmt.Fprintf(out, "| `%s` | %s |\n", name, modes[name]); err != nil {
			return err
		}
	}

	return nil
}

// defaultThemeName reports the theme selected when none is configured, read
// from the kong default declared on the Tui.Theme flag so the name is defined
// exactly once, on the flag.
func defaultThemeName() (string, error) {
	field, ok := reflect.TypeFor[flags.Tui]().FieldByName("Theme")
	if !ok {
		return "", fmt.Errorf("flags.Tui has no Theme field")
	}

	name := field.Tag.Get("default")
	if name == "" {
		return "", fmt.Errorf("Tui.Theme has no kong default")
	}

	return name, nil
}
