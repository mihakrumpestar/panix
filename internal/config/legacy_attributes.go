package config

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/mihakrumpestar/panix/pkg/yamlx"
	"github.com/pkg/errors"
)

// legacyAutoRollbackKey is the attribute removed by the Activation Guard
// (docs/design/activation-guard.md section 2): it is replaced outright by the
// rollback tier selector, with no compatibility mode.
const legacyAutoRollbackKey = "auto_rollback"

// legacyAutoRollbackHint is the migration hint attached to every rejection.
const legacyAutoRollbackHint = "replace auto_rollback: true with rollback: magic, auto_rollback: false with rollback: off"

// rejectLegacyKeys scans the template-processed YAML for attributes removed in
// breaking releases and fails with a migration hint before the strict decode
// would report them as unknown fields. It walks the whole document instead of
// the four attribute levels (fleet, flake, installable, machine): the removed
// names are unique to their former struct locations, so any occurrence is a
// legacy usage.
func rejectLegacyKeys(processedYAML []byte) error {
	var doc any

	err := yamlx.Decode(processedYAML, &doc)
	if err != nil {
		// Not decodable as generic YAML: the strict decode of the actual
		// configuration produces the authoritative syntax error.
		return nil
	}

	var paths []string
	collectLegacyKeyPaths(doc, "", &paths)

	if len(paths) == 0 {
		return nil
	}

	return errors.Errorf(
		"legacy attribute %q is no longer supported at: %s\n%s",
		legacyAutoRollbackKey, strings.Join(paths, ", "), legacyAutoRollbackHint,
	)
}

// collectLegacyKeyPaths appends the dotted YAML path of every occurrence of
// legacyAutoRollbackKey to paths. Map iteration follows the document order
// (yamlx decodes into yaml.MapSlice), so the reported paths are deterministic.
func collectLegacyKeyPaths(node any, path string, paths *[]string) {
	switch typed := node.(type) {
	case yaml.MapSlice:
		for _, item := range typed {
			key := fmt.Sprintf("%v", item.Key)

			childPath := key
			if path != "" {
				childPath = path + "." + key
			}

			if key == legacyAutoRollbackKey {
				*paths = append(*paths, childPath)
			}

			collectLegacyKeyPaths(item.Value, childPath, paths)
		}
	case []any:
		for i, value := range typed {
			collectLegacyKeyPaths(value, fmt.Sprintf("%s[%d]", path, i), paths)
		}
	}
}
