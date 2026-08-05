package inspect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:funlen
func TestParseNixEnvGenerations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantCurrent   uint
		wantAvailable []uint
		wantDate      string
	}{
		{
			"typical output with current",
			"1   2026-08-01 15:00:44\n" +
				"2   2026-08-01 15:10:22   (current)\n",
			2, []uint{1, 2}, "2026-08-01 15:10:22",
		},
		{
			"single generation current",
			"1   2026-08-01 15:00:44   (current)\n",
			1, []uint{1}, "2026-08-01 15:00:44",
		},
		{
			"multiple generations last current",
			"1   2026-08-01 14:00:00\n" +
				"2   2026-08-01 15:00:00\n" +
				"3   2026-08-01 16:00:00   (current)\n",
			3, []uint{1, 2, 3}, "2026-08-01 16:00:00",
		},
		{
			"no current marker defaults to last",
			"1   2026-08-01 14:00:00\n" +
				"2   2026-08-01 15:00:00\n",
			2, []uint{1, 2}, "",
		},
		{
			"empty output",
			"",
			0, nil, "",
		},
		{
			"whitespace only",
			"   \n   \n   ",
			0, nil, "",
		},
		{
			"non-numeric first field skipped",
			"header line\n1   2026-08-01 15:00:00   (current)\n",
			1, []uint{1}, "2026-08-01 15:00:00",
		},
		{
			"non-sequential generation numbers",
			"5   2026-08-01 14:00:00\n" +
				"10   2026-08-01 15:00:00   (current)\n" +
				"100   2026-08-01 16:00:00\n",
			10, []uint{5, 10, 100}, "2026-08-01 15:00:00",
		},
		{
			"with extra blank lines",
			"\n\n1   2026-08-01 15:00:00   (current)\n\n",
			1, []uint{1}, "2026-08-01 15:00:00",
		},
		{
			"multiple current markers last wins",
			"1   2026-08-01 14:00:00   (current)\n" +
				"2   2026-08-01 15:00:00   (current)\n",
			2, []uint{1, 2}, "2026-08-01 15:00:00",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)

			gen, date := parseNixEnvGenerations(test.input)

			if gen == nil {
				assertion.Nil(test.wantAvailable)
				assertion.Equal(uint(0), test.wantCurrent)
				assertion.Equal(test.wantDate, date)

				return
			}

			assertion.Equal(test.wantCurrent, gen.Current)
			assertion.Equal(test.wantAvailable, gen.Available)
			assertion.Equal(test.wantDate, date)
		})
	}
}

func TestParseNixEnvGenerations_EmptyReturnsNil(t *testing.T) {
	t.Parallel()

	gen, date := parseNixEnvGenerations("")

	require.Nil(t, gen)
	assertion := assert.New(t)
	assertion.Empty(date)
}
