package inspect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:funlen
func TestParseGenerationLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		lineNum     int
		line        string
		wantGenNum  uint
		wantDate    string
		wantNixos   string
		wantKernel  string
		wantCurrent bool
		wantOK      bool
	}{
		{
			"header line skipped",
			0,
			"  generation   date        time   nixos     kernel   path",
			0, "", "", "", false, false,
		},
		{"empty line skipped", 1, "", 0, "", "", "", false, false},
		{"whitespace only skipped", 1, "   ", 0, "", "", "", false, false},
		{"non-numeric first field", 1, "abc def ghi", 0, "", "", "", false, false},
		{
			"current gen with true marker",
			1,
			"1   2025-04-19   10:00:00   24.11   6.6.0   /nix/store/abc-system   True",
			1, "2025-04-19", "24.11", "6.6.0", true, true,
		},
		{
			"non-current gen with false marker",
			2,
			"2   2025-04-18   09:00:00   24.11   6.6.0   /nix/store/def-system   False",
			2, "2025-04-18", "24.11", "6.6.0", false, true,
		},
		{
			"gen without current marker",
			1,
			"5   2025-04-17   08:00:00   24.11   6.6.0   /nix/store/xyz-system",
			5, "2025-04-17", "24.11", "6.6.0", false, true,
		},
		{"minimal fields gen num only", 1, "42", 42, "", "", "", false, true},
		{"two fields gen and date", 1, "10   2025-04-19", 10, "2025-04-19", "", "", false, true},
		{
			"four fields gen date time nixos",
			1,
			"7   2025-04-19   12:00:00   24.05",
			7, "2025-04-19", "24.05", "", false, true,
		},
		{
			"five fields includes kernel",
			1,
			"8   2025-04-19   12:00:00   24.11   6.6.0",
			8, "2025-04-19", "24.11", "6.6.0", false, true,
		},
		{
			"extra whitespace handled",
			1,
			"3    2025-04-19    10:00:00    24.05    6.1.0    /nix/store/path    True",
			3, "2025-04-19", "24.05", "6.1.0", true, true,
		},
		{
			"large generation number",
			1,
			"999999   2025-04-19   00:00:00   24.11   6.6.0   /path   False",
			999999, "2025-04-19", "24.11", "6.6.0", false, true,
		},
		{
			"six fields without true not current",
			1,
			"1   2025-04-19   10:00:00   24.11   6.6.0   /nix/store/path",
			1, "2025-04-19", "24.11", "6.6.0", false, true,
		},
		{
			"seven fields with true is current",
			1,
			"1   2025-04-19   10:00:00   24.11   6.6.0   /nix/store/path   True",
			1, "2025-04-19", "24.11", "6.6.0", true, true,
		},
		{
			"true not as last field not current",
			1,
			"1   True   2025-04-19   12:00:00   24.11   6.6.0   /nix/store/path",
			1, "True", "12:00:00", "24.11", false, true,
		},
		{"line zero with valid data skipped", 0, "5   2025-04-19   10:00:00   24.11   6.6.0   /path   True", 0, "", "", "", false, false},
		{"negative gen number skipped", 1, "-1   2025-04-19   10:00:00   24.11   6.6.0", 0, "", "", "", false, false},
		{"zero gen number", 1, "0   2025-04-19   10:00:00   24.11   6.6.0", 0, "2025-04-19", "24.11", "6.6.0", false, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)

			genNum, info, isCurrent, ok := parseGenerationLine(test.lineNum, test.line)

			assertion.Equal(test.wantOK, ok)

			if !test.wantOK {
				assertion.Zero(genNum)
				assertion.False(isCurrent)

				return
			}

			require.True(t, ok)
			assertion.Equal(test.wantGenNum, genNum)
			assertion.Equal(test.wantCurrent, isCurrent)
			assertion.Equal(test.wantDate, info.Date)
			assertion.Equal(test.wantNixos, info.Nixos)
			assertion.Equal(test.wantKernel, info.Kernel)
		})
	}
}

//nolint:funlen
func TestParseGenerationsOutput(t *testing.T) {
	t.Parallel()

	const headerLine = "  generation   date        time      nixos     kernel   path"

	tests := []struct {
		name            string
		input           string
		wantCurrent     uint
		wantAvailable   []uint
		wantDate        string
		wantNixos       string
		wantKernel      string
		wantErr         bool
		wantErrContains string
	}{
		{
			"typical output with current",
			headerLine + "\n" +
				"1   2025-04-19   10:00:00   24.11   6.6.0   /nix/store/abc-system   True\n" +
				"2   2025-04-18   09:00:00   24.11   6.6.0   /nix/store/def-system   False",
			1, []uint{1, 2}, "2025-04-19", "24.11", "6.6.0", false, "",
		},
		{
			"single generation",
			headerLine + "\n" +
				"1   2025-04-19   10:00:00   24.11   6.6.0   /nix/store/abc-system   True",
			1, []uint{1}, "2025-04-19", "24.11", "6.6.0", false, "",
		},
		{
			"multiple generations last current",
			headerLine + "\n" +
				"1   2025-04-17   08:00:00   24.11   6.6.0   /nix/store/a-system   False\n" +
				"2   2025-04-18   09:00:00   24.11   6.6.0   /nix/store/b-system   False\n" +
				"3   2025-04-19   10:00:00   24.11   6.6.0   /nix/store/c-system   True",
			3, []uint{1, 2, 3}, "2025-04-19", "24.11", "6.6.0", false, "",
		},
		{
			"no true false marker",
			headerLine + "\n" +
				"1   2025-04-19   10:00:00   24.11   6.6.0   /nix/store/a-system\n" +
				"2   2025-04-18   09:00:00   24.11   6.6.0   /nix/store/b-system",
			0, []uint{1, 2}, "", "", "", false, "",
		},
		{"empty output", "", 0, nil, "", "", "", true, "no generations found"},
		{"header only no generations", headerLine + "\n", 0, nil, "", "", "", true, "no generations found"},
		{"whitespace only", "   \n   \n   ", 0, nil, "", "", "", true, "no generations found"},
		{
			"with extra blank lines",
			headerLine + "\n\n" +
				"1   2025-04-19   10:00:00   24.11   6.6.0   /nix/store/system   True\n\n",
			1, []uint{1}, "2025-04-19", "24.11", "6.6.0", false, "",
		},
		{
			"non-sequential generation numbers",
			headerLine + "\n" +
				"5   2025-04-19   10:00:00   24.11   6.6.0   /nix/store/e-system   True\n" +
				"10   2025-04-18   09:00:00   24.11   6.6.0   /nix/store/j-system   False\n" +
				"100   2025-04-17   08:00:00   24.11   6.6.0   /nix/store/h-system   False",
			5, []uint{5, 10, 100}, "2025-04-19", "24.11", "6.6.0", false, "",
		},
		{
			"mixed current flags",
			headerLine + "\n" +
				"1   2025-04-17   08:00:00   24.11   6.1.0   /nix/store/a   False\n" +
				"2   2025-04-18   09:00:00   24.05   6.1.0   /nix/store/b   True\n" +
				"3   2025-04-19   10:00:00   24.11   6.6.0   /nix/store/c   False",
			2, []uint{1, 2, 3}, "2025-04-18", "24.05", "6.1.0", false, "",
		},
		{"all malformed lines", "not a generation line\nalso not valid", 0, nil, "", "", "", true, "no generations found"},
		{
			"multiple true markers last wins",
			headerLine + "\n" +
				"1   2025-04-17   08:00:00   24.11   6.1.0   /nix/store/a   True\n" +
				"2   2025-04-18   09:00:00   24.05   6.1.0   /nix/store/b   True",
			2, []uint{1, 2}, "2025-04-18", "24.05", "6.1.0", false, "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)

			gen, info, err := parseGenerationsOutput(test.input)

			if test.wantErr {
				require.Error(t, err)
				assertion.Contains(err.Error(), test.wantErrContains)

				return
			}

			require.NoError(t, err)
			assertion.Equal(test.wantCurrent, gen.Current)
			assertion.Equal(test.wantAvailable, gen.Available)
			assertion.Equal(test.wantDate, info.Date)
			assertion.Equal(test.wantNixos, info.Nixos)
			assertion.Equal(test.wantKernel, info.Kernel)
		})
	}
}
