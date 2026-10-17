package theme

import (
	"embed"
	"path"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/mihakrumpestar/panix/pkg/yamlx"
)

// baseThemeName is the embedded theme every partial theme merges onto: the full
// built-in theme in builtin/panix.yml. The theme selected when none is configured
// is the kong `default:"panix"` tag on Flags.Tui.Theme.
const baseThemeName = "panix"

// builtinDir is the committed embedded built-in theme directory: one
// <name>.yml per built-in theme plus panix.yml for the full default, the single
// source of truth for built-in themes.
const builtinDir = "builtin"

//go:embed builtin/*.yml
var builtinFS embed.FS

// embeddedThemes caches the parsed embedded built-in themes. Parsing and
// validation run exactly once; every accessor returns a deep copy so callers
// cannot mutate the cache.
var (
	embeddedThemesOnce sync.Once
	embeddedThemes     map[string]Theme
	errEmbeddedThemes  error
)

// loadEmbeddedThemes parses every embedded builtin/*.yml exactly once and returns
// the name-keyed themes. The returned map is the shared cache: callers must not
// mutate it (use the accessors, which deep copy). Failures are cached too.
func loadEmbeddedThemes() (map[string]Theme, error) {
	embeddedThemesOnce.Do(func() {
		entries, err := builtinFS.ReadDir(builtinDir)
		if err != nil {
			errEmbeddedThemes = errors.Wrap(err, "read embedded built-in themes")

			return
		}

		themes := make(map[string]Theme, len(entries))

		for _, entry := range entries {
			name := strings.TrimSuffix(entry.Name(), ".yml")

			data, readErr := builtinFS.ReadFile(path.Join(builtinDir, entry.Name()))
			if readErr != nil {
				errEmbeddedThemes = errors.Wrapf(readErr, "read embedded theme %q", name)

				return
			}

			var theme Theme

			err = yamlx.Decode(data, &theme)
			if err != nil {
				errEmbeddedThemes = errors.Wrapf(err, "decode embedded theme %q", name)

				return
			}

			theme, err = prepareTheme(theme)
			if err != nil {
				errEmbeddedThemes = errors.Wrapf(err, "validate embedded theme %q", name)

				return
			}

			themes[name] = theme
		}

		embeddedThemes = themes
	})

	return embeddedThemes, errEmbeddedThemes
}

// mustEmbeddedThemes returns the embedded built-in theme cache, panicking when
// the committed data cannot be parsed or validated.
func mustEmbeddedThemes() map[string]Theme {
	themes, err := loadEmbeddedThemes()
	if err != nil {
		panic(errors.Wrap(err, "load embedded built-in themes"))
	}

	return themes
}

// builtinThemes returns the embedded built-in themes keyed by name, including the
// full "panix" default. Every value is a deep copy.
func builtinThemes() map[string]Theme {
	themes := mustEmbeddedThemes()
	builtins := make(map[string]Theme, len(themes))

	for name, theme := range themes {
		builtins[name] = theme.clone()
	}

	return builtins
}

// builtinTheme returns a deep copy of the embedded theme name. ok is
// false when the name is unknown.
func builtinTheme(name string) (Theme, bool) {
	theme, ok := mustEmbeddedThemes()[name]
	if !ok {
		return Theme{}, false
	}

	return theme.clone(), true
}

// BuiltinThemeModes returns the embedded built-in theme name to terminal
// background mode ("dark" or "light"). The returned map is fresh on every call.
func BuiltinThemeModes() map[string]string {
	themes := mustEmbeddedThemes()

	modes := make(map[string]string, len(themes))
	for name, theme := range themes {
		modes[name] = theme.Mode
	}

	return modes
}

// defaultTheme returns a deep copy of the embedded full "panix" theme: the
// merge base for every partial theme and the source of DefaultTheme.
func defaultTheme() Theme {
	theme, ok := builtinTheme(baseThemeName)
	if !ok {
		panic(errors.Errorf("embedded theme %q is missing", baseThemeName))
	}

	return theme
}
