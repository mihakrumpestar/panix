package phaseops

import (
	"path/filepath"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
)

// OutLinks is the build outlinks configuration shared by phase handlers.
type OutLinks struct {
	Enabled bool
	Dir     string
}

// ClosurePath returns the --out-link path for the installable's closure,
// or "" when outlinks are disabled.
func (o OutLinks) ClosurePath(inst *installable.Installable) string {
	return o.path(inst, "")
}

// DiskoPath returns the --out-link path for the installable's disko script
// (suffixed to not collide with the closure outlink), or "" when outlinks
// are disabled.
func (o OutLinks) DiskoPath(inst *installable.Installable) string {
	return o.path(inst, "-disko")
}

func (o OutLinks) path(inst *installable.Installable, suffix string) string {
	if !o.Enabled {
		return ""
	}

	return filepath.Join(o.Dir, inst.Xpath.String()+suffix)
}
