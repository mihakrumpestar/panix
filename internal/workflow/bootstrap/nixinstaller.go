package bootstrap

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

var ErrNixNotInstalled = errors.New("nix is not available after running the installer")

// nixInstallerPath is where the fetched installer (a shell script with a
// shebang, or an installer binary) is staged on the target before it is run
// elevated.
const nixInstallerPath = "/tmp/panix-nix-installer"

// installNix installs Nix on a target that lacks it: stage the installer,
// run it elevated with the configured args, then verify nix works.
func installNix(exc *executioner.Executioner, machineI *machine.Machine) error {
	installerURL := machineI.Bootstrap.Nix.GetURL()

	err := fetchNixInstaller(exc, machineI, installerURL)
	if err != nil {
		return err
	}

	err = runNixInstaller(exc, machineI)
	if err != nil {
		return err
	}

	return verifyNixInstallation(exc, machineI)
}

// fetchNixInstaller stages the installer at nixInstallerPath and makes it
// executable, so the fetch contract is "staged and runnable" for both a
// shebang script and an installer binary.
func fetchNixInstaller(exc *executioner.Executioner, machineI *machine.Machine, source string) error {
	err := downloadOrTransfer(exc, machineI, source, nixInstallerPath, "nix installer", machineI.Bootstrap.Nix.GetCurlDefaultFlags())
	if err != nil {
		return err
	}

	// Unprivileged is fine: the staged file is owned by the SSH user.
	err = exc.Exec(
		"make nix installer executable",
		"making nix installer executable",
		"failed to make nix installer executable",
		[]string{"chmod", "+x", nixInstallerPath},
	)
	if err != nil {
		return errors.Wrap(err, "nix installer chmod failed")
	}

	return nil
}

// nixInstallerRunArgv returns the argv that runs the staged installer with
// the configured args (fully replacing the defaults), elevated like disko:
// the installer writes the nix store and profiles system-wide. The staged
// file is exec'd directly: a shell script runs via its shebang, an installer
// binary runs natively.
func nixInstallerRunArgv(machineI *machine.Machine) []string {
	return slices.Concat(
		machineI.MaybeSudo(),
		[]string{nixInstallerPath},
		machineI.Bootstrap.Nix.GetArgs(),
	)
}

// runNixInstaller executes the staged installer.
func runNixInstaller(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := exc.Exec(
		"run nix installer",
		"installing nix",
		"nix installer exited with an error",
		nixInstallerRunArgv(machineI),
		executioner.Trim(),
	)
	if err != nil {
		return errors.Wrap(err, "nix installer run failed")
	}

	return nil
}

// verifyNixInstallation re-probes nix and refreshes MetaInspect.NixAvailable.
func verifyNixInstallation(exc *executioner.Executioner, machineI *machine.Machine) error {
	err := phaseops.ProbeNixAvailable(exc, machineI)
	if err != nil {
		return errors.Wrapf(ErrNixNotInstalled, "%s", err.Error())
	}

	return nil
}
