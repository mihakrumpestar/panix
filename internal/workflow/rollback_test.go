package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
)

func TestValidateAndGetTargetGen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		current   uint
		available []uint
		rollback  int
		wantGen   uint
		wantErrIs error
	}{
		{"zero rollback returns current", 5, []uint{1, 2, 3, 4, 5}, 0, 5, nil},
		{"negative one returns previous", 5, []uint{1, 2, 3, 4, 5}, -1, 4, nil},
		{"negative two returns two back", 5, []uint{1, 2, 3, 4, 5}, -2, 3, nil},
		{"negative four from five equals one", 5, []uint{1, 2, 3, 4, 5}, -4, 1, nil},
		{"negative overflow returns error", 2, []uint{1, 2}, -5, 0, ErrGenerationOutOfRange},
		{"negative to zero returns error", 3, []uint{1, 2, 3}, -3, 0, ErrGenerationOutOfRange},
		{"positive exact generation", 5, []uint{1, 2, 3, 4, 5}, 3, 3, nil},
		{"positive one returns one", 5, []uint{1, 2, 3, 4, 5}, 1, 1, nil},
		{"positive equals current", 5, []uint{1, 2, 3, 4, 5}, 5, 5, nil},
		{"positive not in available", 5, []uint{1, 2, 3, 4, 5}, 99, 0, ErrGenerationOutOfRange},
		{"positive zero returns current", 5, []uint{1, 2, 3, 4, 5}, 0, 5, nil},
		{"current at one with neg one", 1, []uint{1}, -1, 0, ErrGenerationOutOfRange},
		{"current at ten with neg offset", 10, []uint{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, -5, 5, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)

			gen := &machine.Generations{Current: test.current, Available: test.available}
			result, err := validateAndGetTargetGen(gen, test.rollback)

			if test.wantErrIs != nil {
				require.ErrorIs(t, err, test.wantErrIs)

				return
			}

			require.NoError(t, err)
			assertion.Equal(test.wantGen, result)
		})
	}
}

func TestValidateAndGetTargetGenErrorMessage(t *testing.T) {
	t.Parallel()

	gen := &machine.Generations{Current: 5, Available: []uint{1, 2, 3, 4, 5}}
	_, err := validateAndGetTargetGen(gen, 99)

	require.ErrorIs(t, err, ErrGenerationOutOfRange)
	assert.Contains(t, err.Error(), "generation 99 not found")
}

func TestValidateAndGetTargetGenNegativeToZeroMessage(t *testing.T) {
	t.Parallel()

	gen := &machine.Generations{Current: 3, Available: []uint{1, 2, 3}}
	_, err := validateAndGetTargetGen(gen, -3)

	require.ErrorIs(t, err, ErrGenerationOutOfRange)
	assert.Contains(t, err.Error(), "0")
}

func TestGetSpecificGeneration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		available []uint
		target    uint
		wantGen   uint
		wantErrIs error
	}{
		{"generation in available", []uint{1, 2, 3, 4, 5}, 3, 3, nil},
		{"first generation", []uint{1, 2, 3}, 1, 1, nil},
		{"last generation", []uint{1, 2, 3}, 3, 3, nil},
		{"generation not in available", []uint{1, 2, 3}, 99, 0, ErrGenerationOutOfRange},
		{"empty available list", []uint{}, 1, 0, ErrGenerationOutOfRange},
		{"single generation available", []uint{42}, 42, 42, nil},
		{"single generation not matching", []uint{42}, 1, 0, ErrGenerationOutOfRange},
		{"non sequential generations", []uint{5, 10, 15, 20}, 10, 10, nil},
		{"non sequential not found", []uint{5, 10, 15, 20}, 7, 0, ErrGenerationOutOfRange},
		{"zero target not in list", []uint{1, 2, 3}, 0, 0, ErrGenerationOutOfRange},
		{"zero target in list", []uint{0, 1, 2}, 0, 0, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)

			result, err := getSpecificGeneration(test.available, test.target)

			if test.wantErrIs != nil {
				require.ErrorIs(t, err, test.wantErrIs)

				return
			}

			require.NoError(t, err)
			assertion.Equal(test.wantGen, result)
		})
	}
}

func TestGetSpecificGenerationErrorMessage(t *testing.T) {
	t.Parallel()

	_, err := getSpecificGeneration([]uint{5, 10, 15, 20}, 7)

	require.ErrorIs(t, err, ErrGenerationOutOfRange)
	assert.Contains(t, err.Error(), "generation 7 not found")
}
