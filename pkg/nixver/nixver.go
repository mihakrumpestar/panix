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
	Raw    string `json:"-"` // raw output from "nix --version"
	Flavor Flavor `json:"-"` // parsed flavor: Nix or Lix
	Major  int    `json:"-"`
	Minor  int    `json:"-"`
	Patch  int    `json:"-"`
}

// Detect runs "nix --version" and returns the parsed implementation info.
func Detect() (*Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "nix", "--version").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to detect nix version")
	}

	return parseNixVersion(string(out)), nil // Parsing errors are not threated as real error, since we can run without Nix flavor detection too
}

// Version returns the semver string, e.g. "2.94.0".
func (i *Info) Version() string {
	if i.Major == 0 && i.Minor == 0 && i.Patch == 0 {
		return ""
	}

	return strconv.Itoa(i.Major) + "." + strconv.Itoa(i.Minor) + "." + strconv.Itoa(i.Patch)
}

// String returns the raw nix --version output.
func (i *Info) String() string {
	return i.Raw
}

// MarshalJSON outputs just the raw nix --version string.
func (i *Info) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(i.Raw)
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

	*i = *parseNixVersion(raw)

	return nil
}
