package nixver

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

// Flavor represents the Nix implementation flavor.
type Flavor string

const (
	FlavorNix Flavor = "Nix"
	FlavorLix Flavor = "Lix"
)

// Info holds detected Nix implementation information.
type Info struct {
	raw    string `json:"-"` // raw output from "nix --version"
	flavor Flavor `json:"-"` // parsed flavor: Nix or Lix
	major  int    `json:"-"`
	minor  int    `json:"-"`
	patch  int    `json:"-"`
}

// GetFlavor returns the detected nix flavor, defaulting to FlavorNix when
// the Info pointer is nil (e.g. tests that bypass the config loader).
func (i *Info) GetFlavor() Flavor {
	if i == nil {
		return FlavorNix
	}

	return i.flavor
}

func (i *Info) GetRaw() string {
	return i.raw
}

// Detect runs "nix --version" and returns the parsed implementation info.
func Detect() (*Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "nix", "--version").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect nix version")
	}

	return ParseNixVersion(string(out)), nil // Parsing errors are not threated as real error, since we can run without Nix flavor detection too
}

// Version returns the semver string, e.g. "2.94.0".
func (i *Info) Version() string {
	if i.major == 0 && i.minor == 0 && i.patch == 0 {
		return ""
	}

	return strconv.Itoa(i.major) + "." + strconv.Itoa(i.minor) + "." + strconv.Itoa(i.patch)
}

// String returns the raw nix --version output.
func (i *Info) String() string {
	return i.raw
}

// MarshalJSON outputs just the raw nix --version string.
func (i *Info) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(i.raw)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal nix info")
	}

	return data, nil
}

// UnmarshalJSON parses the raw nix --version string back into Info.
func (i *Info) UnmarshalJSON(data []byte) error {
	var raw string

	err := json.Unmarshal(data, &raw)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal nix info")
	}

	*i = *ParseNixVersion(raw)

	return nil
}
