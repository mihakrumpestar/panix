package gen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/boyter/scc/v3/processor"
	"github.com/narqo/go-badge"
	"github.com/pkg/errors"
)

var (
	codebaseLanguages = []string{"Go", "Nix"}
)

type langCode struct {
	Code int64  `json:"Code"`
	Name string `json:"Name"`
}

func LocBadge() error {
	workDir, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "get working directory")
	}

	root := filepath.Join(workDir, "..")

	tmp, err := os.CreateTemp("", "scc-*.json")
	if err != nil {
		return errors.Wrap(err, "create temp file")
	}

	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) //nolint:errcheck

	processor.DirFilePaths = []string{root}
	processor.Format = "json"
	processor.FileOutput = tmpPath

	devNull, _ := os.Open(os.DevNull)
	oldStdout := os.Stdout
	os.Stdout = devNull

	processor.Process()

	os.Stdout = oldStdout

	out, err := os.ReadFile(tmpPath) //nolint:gosec
	if err != nil {
		return errors.Wrap(err, "read scc output")
	}

	var langs []langCode

	err = json.Unmarshal(out, &langs)
	if err != nil {
		return errors.Wrap(err, "unmarshal scc output")
	}

	var total int64

	for _, l := range langs {
		if slices.Contains(codebaseLanguages, l.Name) {
			total += l.Code
		}
	}

	file, err := os.Create(filepath.Join(workDir, "loc.svg")) //nolint:gosec
	if err != nil {
		return errors.Wrap(err, "create loc.svg")
	}
	defer file.Close() //nolint:errcheck

	return errors.Wrap(badge.Render("loc", humanInt(total), badge.Color("#5277C3"), file), "render badge")
}

func humanInt(num int64) string {
	switch {
	case num >= 1_000_000: //nolint:mnd
		return trimZ(fmt.Sprintf("%.1fM", float64(num)/1e6)) //nolint:mnd
	case num >= 1_000: //nolint:mnd
		return trimZ(fmt.Sprintf("%.1fK", float64(num)/1e3)) //nolint:mnd
	default:
		return strconv.FormatInt(num, 10)
	}
}

func trimZ(s string) string {
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}
