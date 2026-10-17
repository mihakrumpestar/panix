package theme

import (
	"maps"
	"slices"

	"github.com/pkg/errors"
)

// SourceBuiltin is the Registry.Source label for built-in themes.
const SourceBuiltin = "builtin"

// Registry resolves theme names to Themes. It holds the built-in themes plus
// optional user themes loaded from the local and global panix-themes.yml files
// (merged, local wins field by field). A user theme with the same name shadows the
// built-in one; built-ins remain available under their other names.
type Registry struct {
	entries map[string]registryEntry
}

type registryEntry struct {
	theme  Theme
	source string
}

// NewRegistry returns a registry preloaded with the built-in themes. User themes
// are added via LoadUserThemes or LoadThemesFiles.
func NewRegistry() *Registry {
	builtins := builtinThemes()
	registry := &Registry{entries: make(map[string]registryEntry, len(builtins))}

	for name, theme := range builtins {
		registry.entries[name] = registryEntry{theme: theme, source: SourceBuiltin}
	}

	return registry
}

// LoadUserThemes resolves the user themes files (see ResolveThemesFiles) and adds
// their merged themes to the registry. explicitPath replaces the implicit local
// candidate when non-empty. Any problem with a themes file is a hard config error:
// theme problems fail at eval.
func (r *Registry) LoadUserThemes(explicitPath string) error {
	local, global, err := ResolveThemesFiles(explicitPath)
	if err != nil {
		return errors.Wrap(err, "themes file lookup")
	}

	if local == "" && global == "" {
		return nil
	}

	return r.LoadThemesFiles(local, global)
}

// LoadThemesFiles parses the global and local user themes files (either path may
// be "" when absent) and adds their merged themes to the registry; entries shadow
// built-ins with the same name. A theme defined in both files merges field by
// field with the local theme winning (non-zero local fields override, unset local
// fields inherit, see Theme.Merge). Provenance is the local path for every name
// the local file defines (even partially) and the global path for the rest. Any
// problem in either file is a hard error; the registry is only extended after both
// files parsed and validated cleanly.
func (r *Registry) LoadThemesFiles(localPath, globalPath string) error {
	globalThemes := map[string]Theme{}

	if globalPath != "" {
		var err error

		globalThemes, err = ParseThemesFile(globalPath)
		if err != nil {
			return err
		}
	}

	localThemes := map[string]Theme{}

	if localPath != "" {
		var err error

		localThemes, err = ParseThemesFile(localPath)
		if err != nil {
			return err
		}
	}

	for name, theme := range globalThemes {
		r.entries[name] = registryEntry{theme: theme, source: globalPath}
	}

	for name, localTheme := range localThemes {
		r.entries[name] = registryEntry{theme: globalThemes[name].Merge(localTheme), source: localPath}
	}

	return nil
}

// Names returns all theme names (built-in and user), sorted. A user theme
// shadowing a built-in appears once.
func (r *Registry) Names() []string {
	return slices.Sorted(maps.Keys(r.entries))
}

// Has reports whether name is a known theme (built-in or user) without resolving
// it. It is the cheap existence check for callers that only validate a name.
func (r *Registry) Has(name string) bool {
	_, ok := r.entries[name]

	return ok
}

// Build resolves name into a fresh *Theme. Partial themes are merged onto
// defaultTheme first, so only their set fields differ from the default look. The
// merged value is deep-copied so it never aliases the registry. ok is false when
// the name is unknown.
func (r *Registry) Build(name string) (*Theme, bool) {
	entry, ok := r.entries[name]
	if !ok {
		return nil, false
	}

	merged := defaultTheme().Merge(entry.theme).clone()

	return &merged, true
}

// Source returns the provenance of a theme for display: SourceBuiltin, the local
// user themes file path when that file defines the name (even partially, the
// highest precedence contributor), or the global path otherwise. Empty string for
// unknown names.
func (r *Registry) Source(name string) string {
	entry, ok := r.entries[name]
	if !ok {
		return ""
	}

	return entry.source
}
