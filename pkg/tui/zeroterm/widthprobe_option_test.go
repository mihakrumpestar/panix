package zeroterm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // package-level globals not concurrency-safe
func TestWithWidthProbeStoresConfig(t *testing.T) {
	glyphs := []string{"🪜", "📋"}

	var gotElapsed time.Duration

	install := func(_ map[string]int, elapsed time.Duration) {
		gotElapsed = elapsed
	}

	program := NewProgram(&dummyModel{})
	WithWidthProbe(glyphs, install)(program)

	assert.Equal(t, glyphs, program.widthProbeGlyphs)
	require.NotNil(t, program.widthProbeInstall)

	program.widthProbeInstall(map[string]int{"🪜": 1}, 42*time.Millisecond)
	assert.Equal(t, 42*time.Millisecond, gotElapsed)
}

//nolint:paralleltest // package-level globals not concurrency-safe
func TestProbeWidths_NoopWithoutConfig(t *testing.T) {
	program := NewProgram(&dummyModel{})

	assert.Nil(t, program.probeWidths())
}
