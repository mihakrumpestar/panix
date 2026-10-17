package theme

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"

	"github.com/pkg/errors"

	"github.com/mihakrumpestar/panix/pkg/yamlx"
)

// ThemesFileName is the implicit user themes file name, looked up in the working
// directory (local slot) and the user config directory (global). A configured
// explicit path replaces the local slot; both files, when present, are loaded and
// merged with local precedence.
const ThemesFileName = "panix-themes.yml"

// themesFile is the on-disk shape of panix-themes.yml: a top level themes map of
// theme name to (partial) Theme.
type themesFile struct {
	Themes map[string]Theme `yaml:"themes" json:"themes" desc:"Theme name to (partial) theme mapping"`
}

// ThemesFileRootType returns the schema root type of the user themes file. The
// unexported themesFile DTO is the single source of truth for its on-disk shape,
// so pkg/yamlschema derives the published schema from the same type and the file
// format cannot drift from it.
func ThemesFileRootType() reflect.Type {
	return reflect.TypeFor[themesFile]()
}

// ResolveThemesFiles returns the two user themes file candidates, local first.
// The local slot is explicitPath when configured (relative to the working
// directory) or the implicit ./panix-themes.yml when it is empty and exists. The
// global candidate is os.UserConfigDir()/panix/panix-themes.yml. Each is "" when
// that location holds no regular file. Both files, when present, merge field by
// field with the local file taking precedence (see Registry.LoadThemesFiles).
//
// Lookup is best-effort: when the working directory or user config directory is
// unavailable the corresponding candidate is empty, so built-in themes and
// absolute explicit paths still work. A relative explicit path genuinely needs
// the working directory and is a hard error when it is unavailable. A configured
// explicitPath must be an existing regular file: a missing path is a hard error
// naming it, and never falls back to the implicit local file.
func ResolveThemesFiles(explicitPath string) (string, string, error) {
	var workDir string

	if explicitPath == "" || !filepath.IsAbs(explicitPath) {
		dir, err := os.Getwd()
		if err != nil {
			if explicitPath != "" {
				return "", "", errors.Wrap(err, "working directory")
			}
		} else {
			workDir = dir
		}
	}

	configDir, _ := os.UserConfigDir()

	return resolveThemesFiles(workDir, configDir, explicitPath)
}

// resolveThemesFiles is the pure resolution rule behind ResolveThemesFiles. The
// local slot is the explicit path or the implicit project file; the global slot is
// empty when it is not a regular file. An empty workDir or configDir disables the
// candidate that needs it. An explicit path that is not a regular file is a hard
// error.
func resolveThemesFiles(workDir, configDir, explicitPath string) (string, string, error) {
	local := explicitPath
	if local != "" {
		if !isRegularFile(local) {
			return "", "", errors.Errorf("themes file %s does not exist or is not a regular file", strconv.Quote(local))
		}
	} else if workDir != "" {
		local = filepath.Join(workDir, ThemesFileName)
		if !isRegularFile(local) {
			local = ""
		}
	}

	global := ""
	if configDir != "" {
		global = filepath.Join(configDir, "panix", ThemesFileName)
		if !isRegularFile(global) {
			global = ""
		}
	}

	return local, global, nil
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)

	return err == nil && !info.IsDir()
}

// ParseThemesFile decodes the themes file at path into named (partial) themes and
// validates every parsed theme before it can be merged. Every problem is a hard
// error naming the file path (and theme name and field path for invalid values):
// theme problems are config errors and fail at eval. An empty file is valid and
// yields no themes. Unknown YAML keys fail exactly like in panix.yml (both decode
// through the strict yamlx.Decode).
func ParseThemesFile(path string) (map[string]Theme, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: user-provided themes file path
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read themes file %s", strconv.Quote(path))
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]Theme{}, nil
	}

	var file themesFile

	err = yamlx.Decode(data, &file)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode themes file %s", strconv.Quote(path))
	}

	if file.Themes == nil {
		return nil, errors.Errorf("themes file %s has no themes (expected a top level %q map)", strconv.Quote(path), "themes")
	}

	for _, name := range slices.Sorted(maps.Keys(file.Themes)) {
		theme, prepareErr := prepareTheme(file.Themes[name])
		if prepareErr != nil {
			return nil, errors.Wrapf(prepareErr, "themes file %s: theme %q", strconv.Quote(path), name)
		}

		file.Themes[name] = theme
	}

	return file.Themes, nil
}
