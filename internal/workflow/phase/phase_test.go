package phase

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhaseString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		phase Phase
		want  string
	}{
		{Inspect, "inspect"},
		{Build, "build"},
		{Bootstrap, "bootstrap"},
		{Transfer, "transfer"},
		{Secrets, "secrets"},
		{Activate, "activate"},
		{Rollback, "rollback"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.phase.String())
		})
	}
}

func TestGetPhaseMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		phase          Phase
		wantFound      bool
		wantScope      PhaseScope
		wantValidFirst bool
	}{
		{"inspect", Inspect, true, ScopeMachine, true},
		{"build", Build, true, ScopeConfiguration, true},
		{"bootstrap", Bootstrap, true, ScopeMachine, false},
		{"transfer", Transfer, true, ScopeMachine, false},
		{"secrets", Secrets, true, ScopeMachine, false},
		{"activate", Activate, true, ScopeMachine, false},
		{"rollback", Rollback, true, ScopeMachine, false},
		{"unknown", Phase("unknown"), false, ScopeMachine, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			meta, found := GetPhaseMetadata(tt.phase)
			assertion.Equal(tt.wantFound, found)

			if found {
				assertion.Equal(tt.wantScope, meta.Scope)
				assertion.Equal(tt.wantValidFirst, meta.ValidFirst)
			}
		})
	}
}

func TestGetPhaseScope(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)
	assertion.Equal(ScopeMachine, Inspect.GetPhaseScope())
	assertion.Equal(ScopeConfiguration, Build.GetPhaseScope())
	assertion.Equal(ScopeMachine, Phase("unknown").GetPhaseScope())
}

func TestShouldRunOnce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		phase Phase
		want  bool
	}{
		{Inspect, false},
		{Build, true},
		{Bootstrap, false},
		{Transfer, false},
		{Secrets, false},
		{Activate, false},
		{Rollback, false},
		{Phase("unknown"), false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.phase.ShouldRunOnce())
		})
	}
}

func TestPhasesInOrder(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)

	expected := []Phase{Inspect, Build, Bootstrap, Transfer, Secrets, Activate, Rollback}
	result := PhasesInOrder()
	assertion.Equal(expected, result)
	assertion.Len(result, len(PhaseRegistry))
}

func TestValidFirstPhases(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)
	assertion.Equal([]Phase{Inspect, Build}, validFirstPhases())
}

func TestValidatePhasesValidCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		requested  []Phase
		skip       []Phase
		wantResult []Phase
	}{
		{
			"normal deploy preserves order",
			[]Phase{Inspect, Build, Activate}, nil,
			[]Phase{Inspect, Build, Activate},
		},
		{
			"out of order input reordered",
			[]Phase{Activate, Inspect}, nil,
			[]Phase{Inspect, Activate},
		},
		{
			"single valid first phase",
			[]Phase{Inspect}, nil,
			[]Phase{Inspect},
		},
		{
			"build as first phase valid",
			[]Phase{Build, Activate}, nil,
			[]Phase{Build, Activate},
		},
		{
			"skip phases removes them",
			[]Phase{Inspect, Build, Bootstrap, Activate},
			[]Phase{Bootstrap},
			[]Phase{Inspect, Build, Activate},
		},
		{
			"skip multiple phases",
			[]Phase{Inspect, Build, Bootstrap, Transfer, Secrets, Activate},
			[]Phase{Bootstrap, Transfer},
			[]Phase{Inspect, Build, Secrets, Activate},
		},
		{
			"skipping non-present phases",
			[]Phase{Inspect, Activate}, []Phase{Bootstrap},
			[]Phase{Inspect, Activate},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)

			result, err := ValidatePhases(tt.requested, tt.skip)
			require.NoError(t, err)
			assertion.Equal(tt.wantResult, result)
		})
	}
}

func TestValidatePhasesErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested []Phase
		skip      []Phase
		wantErrIs error
	}{
		{"unknown phase returns error", []Phase{Inspect, Phase("invalid")}, nil, ErrUnknownPhase},
		{"bootstrap first invalid", []Phase{Bootstrap}, nil, ErrInvalidFirstPhase},
		{"transfer first invalid", []Phase{Transfer}, nil, ErrInvalidFirstPhase},
		{"secrets first invalid", []Phase{Secrets}, nil, ErrInvalidFirstPhase},
		{"activate first invalid", []Phase{Activate}, nil, ErrInvalidFirstPhase},
		{"rollback first invalid", []Phase{Rollback}, nil, ErrInvalidFirstPhase},
		{"all phases skipped", []Phase{Inspect}, []Phase{Inspect}, ErrAllPhasesSkipped},
		{"empty requested", []Phase{}, nil, ErrAllPhasesSkipped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ValidatePhases(tt.requested, tt.skip)
			require.ErrorIs(t, err, tt.wantErrIs)
		})
	}
}

func TestValidatePhasesPreservesRegistryOrder(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)

	result, err := ValidatePhases([]Phase{Activate, Build, Inspect}, nil)
	require.NoError(t, err)
	assertion.True(slices.Equal(result, []Phase{Inspect, Build, Activate}))
}
