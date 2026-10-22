package bootstrap

import (
	"path"
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/pkg/errors"
)

var ErrNixNotInstalled = errors.New("nix is not available after running the installer")

const (
	// installerStagingPrefix names the per-run private staging directory.
	installerStagingPrefix = "panix-installer"
	// installerFileName is the staged installer name inside that directory.
	installerFileName = "installer"
)

// installNix installs Nix on a target that lacks it: stage the installer in a
// fresh private directory, run it elevated with the configured args, then
// verify nix works.
func installNix(exc *executioner.Executioner, machineI *machine.Machine) error {
	installerURL := machineI.Bootstrap.Nix.GetURL()

	installerPath, err := fetchNixInstaller(exc, machineI, installerURL)
	if err != nil {
		return err
	}

	// Best effort: the installer already ran, cleanup must not mask its result.
	defer func() { _ = removeStagingDir(exc, machineI, path.Dir(installerPath)) }()

	err = runNixInstaller(exc, machineI, installerPath)
	if err != nil {
		return err
	}

	return verifyNixInstallation(exc, machineI)
}

// fetchNixInstaller creates a unique private staging directory, fetches the
// installer into it, and makes it executable. It returns the staged path so
// the caller can run and clean up exactly this run's artifact.
func fetchNixInstaller(exc *executioner.Executioner, machineI *machine.Machine, source string) (string, error) {
	stagingDir, err := newStagingDirPath(installerStagingPrefix)
	if err != nil {
		return "", err
	}

	err = createPrivateStagingDir(exc, machineI, stagingDir)
	if err != nil {
		return "", err
	}

	cleanup := func() { _ = removeStagingDir(exc, machineI, stagingDir) }

	installerPath := path.Join(stagingDir, installerFileName)

	err = downloadOrTransfer(exc, machineI, source, installerPath, "nix installer", machineI.Bootstrap.Nix.GetCurlDefaultFlags())
	if err != nil {
		cleanup()

		return "", err
	}

	// Unprivileged is fine: the staged file is owned by the SSH user.
	err = exc.Exec(
		"make nix installer executable",
		"making nix installer executable",
		"failed to make nix installer executable",
		[]string{"chmod", "+x", installerPath},
	)
	if err != nil {
		cleanup()

		return "", errors.Wrap(err, "nix installer chmod failed")
	}

	return installerPath, nil
}

// nixInstallerRunArgv returns the argv that runs the staged installer with
// the configured args (fully replacing the defaults), elevated like disko:
// the installer writes the nix store and profiles system-wide. The staged
// file is exec'd directly: a shell script runs via its shebang, an installer
// binary runs natively.
func nixInstallerRunArgv(machineI *machine.Machine, installerPath string) []string {
	return slices.Concat(
		machineI.MaybeSudo(),
		[]string{installerPath},
		machineI.Bootstrap.Nix.GetArgs(),
	)
}

// runNixInstaller executes the staged installer.
func runNixInstaller(exc *executioner.Executioner, machineI *machine.Machine, installerPath string) error {
	err := exc.Exec(
		"run nix installer",
		"installing nix",
		"nix installer exited with an error",
		nixInstallerRunArgv(machineI, installerPath),
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
