package phaselogs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/xpath"
)

// TestNewCommand_DuplicateDescriptionsHaveDistinctXpaths reproduces issue #33:
// an activation followed by its automatic rollback activation uses the same
// description. Command xpaths are viewport identities, so a duplicate collapses
// both commands into one viewport and Ctrl+C/fullscreen read only the rollback.
func TestNewCommand_DuplicateDescriptionsHaveDistinctXpaths(t *testing.T) {
	t.Parallel()

	phaseXpath := xpath.New("flake0", "cfg0", "machine0", "activate")
	phaseLog := NewPhaseLog()

	failed := phaseLog.NewCommand(phaseXpath, "activate", "activating", "activation failed", []string{"activate"}, 0)
	rollback := phaseLog.NewCommand(phaseXpath, "activate", "activating", "activation failed", []string{"rollback"}, 0)

	require.Equal(t, "activate", failed.Description)
	require.Equal(t, "activate", rollback.Description)

	assert.NotEqual(t, failed.Xpath, rollback.Xpath, "command xpaths must be unique within a phase")
	assert.NotEqual(t, failed.LabelXpath, rollback.LabelXpath, "label xpaths must be unique")
	assert.NotEqual(t, failed.OutputXpath, rollback.OutputXpath, "output xpaths must be unique")
	assert.NotEqual(t, failed.ErrorXpath, rollback.ErrorXpath, "error xpaths must be unique")
}

func TestNewCommand_CommandsSharePhasePrefixAndStayOrdered(t *testing.T) {
	t.Parallel()

	phaseXpath := xpath.New("flake0", "cfg0", "machine0", "activate")
	phaseLog := NewPhaseLog()

	first := phaseLog.NewCommand(phaseXpath, "set profile", "setting profile", "set profile failed", []string{"nix"}, 0)
	second := phaseLog.NewCommand(phaseXpath, "activate", "activating", "activation failed", []string{"activate"}, 0)

	assert.Equal(t, phaseXpath.String()+"/0/set profile", first.Xpath.String())
	assert.Equal(t, phaseXpath.String()+"/1/activate", second.Xpath.String())
}
