package config

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/mihakrumpestar/panix/internal/config/theme"
)

// Theme is the theme configuration: the loaded registry (built-ins plus the
// merged user themes files) and the selected theme name.
type Theme struct {
	Registry *theme.Registry
	Name     string
}

// defaultThemeName is the built-in theme selected when a snapshot has no usable
// theme reference. It mirrors the kong default on Flags.Tui.Theme and the base
// theme of the theme package.
const defaultThemeName = "panix"

// ResolveTheme returns the resolved runtime theme for Theme.Name, built fresh
// from Theme.Registry. ok is false when the registry is missing or the name is
// unknown.
func (c *Config) ResolveTheme() (*theme.Theme, bool) {
	if c.Theme.Registry == nil {
		return nil, false
	}

	return c.Theme.Registry.Build(c.Theme.Name)
}

// resolveTheme is the single source of truth for theme resolution, called from
// applyConfigDefaults and PostUnmarshalInit. It builds the registry (built-ins
// plus the merged user themes files), loads the themes file selected by
// Flags.Tui.ThemesFile (empty means implicit ./panix-themes.yml discovery) and
// validates that Flags.Tui.Theme names an existing theme. The resolved theme is
// derived on use via ResolveTheme, never stored.
func (c *Config) resolveTheme() error {
	registry := theme.NewRegistry()

	err := registry.LoadUserThemes(c.Flags.Tui.ThemesFile)
	if err != nil {
		return errors.Wrap(err, "failed to load user themes")
	}

	name := c.Flags.Tui.Theme
	if name == "" {
		return errors.New("theme name must not be empty")
	}

	if !registry.Has(name) {
		return errors.Errorf("unknown theme %q, available themes: %s", name, strings.Join(registry.Names(), ", "))
	}

	c.Theme.Name = name
	c.Theme.Registry = registry

	return nil
}

// resolveSnapshotTheme installs a best-effort theme for a decoded snapshot.
// Snapshots are debugging artifacts: every theme problem is swallowed and the
// built-in default is used instead of failing the load. It still uses the normal
// registry path, so a valid custom theme named in flags.tui.theme resolves when
// its themes file is available. Normal config loading stays strict through
// resolveTheme.
func (c *Config) resolveSnapshotTheme() {
	name := c.Flags.Tui.Theme
	if name == "" {
		c.setDefaultSnapshotTheme()

		return
	}

	registry := theme.NewRegistry()

	err := registry.LoadUserThemes(c.Flags.Tui.ThemesFile)
	if err != nil || !registry.Has(name) {
		c.setDefaultSnapshotTheme()

		return
	}

	c.Theme = Theme{Registry: registry, Name: name}
}

// setDefaultSnapshotTheme resets to a built-in-only registry: a snapshot without a
// usable theme must render the built-in default even when a user themes file
// shadows it.
func (c *Config) setDefaultSnapshotTheme() {
	c.Theme = Theme{Registry: theme.NewRegistry(), Name: defaultThemeName}
}
