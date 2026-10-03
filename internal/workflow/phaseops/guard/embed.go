package guard

import (
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
)

//go:embed all:bin/*
var guardianFS embed.FS

// Static sentinels the embed resolver wraps with the offending value (spec
// 10.3, 15): dynamic errors are banned, so every failure carries a static
// cause.
var (
	errUnsupportedTargetArch = errors.New("unsupported target architecture (embedded variants: amd64, arm64)")
	errUnsupportedTargetOS   = errors.New("unsupported target OS (embedded variants: linux, darwin)")
	errGuardiansNotEmbedded  = errors.New("panix-guard binaries are not embedded; build via the flake (nix build) or populate bin/*.gz (spec 15)")
)

// archAliases maps uname -m spellings to the embedded variant names (spec 10.3).
var archAliases = map[string]string{
	"x86_64":  "amd64",
	"amd64":   "amd64",
	"aarch64": "arm64",
	"arm64":   "arm64",
}

// NormalizeArch maps uname -m style architectures to the embedded variant names.
// Anything else (riscv64, 386, ...) is unsupported: the guarded tier fails fast at
// the embed step instead of transferring a missing binary.
func NormalizeArch(arch string) (string, error) {
	if a, ok := archAliases[arch]; ok {
		return a, nil
	}

	return "", errors.Wrapf(errUnsupportedTargetArch, "architecture %q", arch)
}

// GuardBinary materializes the embedded guardian variant for the target OS and
// architecture as a local temp file (0700) and returns its path for the transfer
// step (spec 10.3, 15). The flake populates bin/*.gz at build time; an empty embed
// directory (dev build without the flake or the task populate step) fails loudly
// here instead of transferring a missing binary.
func GuardBinary(targetOS, targetArch string) (string, error) {
	if targetOS != "linux" && targetOS != "darwin" {
		return "", errors.Wrapf(errUnsupportedTargetOS, "target OS %q", targetOS)
	}

	arch, err := NormalizeArch(targetArch)
	if err != nil {
		return "", err
	}

	entries, derr := guardianFS.ReadDir("bin")
	if derr != nil {
		return "", errors.Wrap(derr, "read embedded guardians")
	}

	variants := 0

	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".gz") {
			variants++
		}
	}

	if variants == 0 {
		return "", errGuardiansNotEmbedded
	}

	name := fmt.Sprintf("panix-guard-%s-%s.gz", targetOS, arch)

	f, err := guardianFS.Open("bin/" + name)
	if err != nil {
		return "", errors.Wrapf(err, "embedded guardian %s", name)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", errors.Wrapf(err, "gunzip %s", name)
	}
	defer gz.Close()

	tmp, err := os.CreateTemp("", "panix-guard-*.bin")
	if err != nil {
		return "", errors.Wrap(err, "materialize guardian")
	}

	if _, err := io.Copy(tmp, gz); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())

		return "", errors.Wrap(err, "materialize guardian")
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())

		return "", errors.Wrap(err, "materialize guardian")
	}

	if err := os.Chmod(tmp.Name(), 0o700); err != nil {
		_ = os.Remove(tmp.Name())

		return "", errors.Wrap(err, "materialize guardian")
	}

	return tmp.Name(), nil
}
