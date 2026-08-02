package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// createCloudInitISO creates a cloud-init seed ISO.
// When withNix is true, nix is installed via the Determinate Systems installer.
// When shutdown is true, the VM shuts down after cloud-init completes (used for image baking).
func createCloudInitISO(outputName, instanceID, hostname, pubKeyPath string, withNix bool, shutdown bool) (string, error) {
	seedPath := filepath.Join(cacheDirPath, outputName)

	cached, err := checkExistingSeed(seedPath, pubKeyPath, withNix, shutdown)
	if err != nil {
		return "", err
	}

	if cached != "" {
		return cached, nil
	}

	return writeCloudInitISO(seedPath, instanceID, hostname, pubKeyPath, withNix, shutdown)
}

func checkExistingSeed(seedPath, pubKeyPath string, withNix bool, shutdown bool) (string, error) {
	if withNix || shutdown {
		return "", nil
	}

	pubKeyContent, err := os.ReadFile(pubKeyPath) //nolint:gosec
	if err != nil {
		return "", errors.Wrap(err, "read pub key")
	}

	pubKey := strings.TrimSpace(string(pubKeyContent))

	existing, readErr := os.ReadFile(seedPath + ".key") //nolint:gosec
	if readErr == nil && string(existing) == pubKey {
		_, statErr := os.Stat(seedPath)
		if statErr == nil {
			return seedPath, nil
		}
	}

	return "", nil
}

func writeCloudInitISO(seedPath, instanceID, hostname, pubKeyPath string, withNix bool, shutdown bool) (string, error) {
	pubKeyContent, err := os.ReadFile(pubKeyPath) //nolint:gosec
	if err != nil {
		return "", errors.Wrap(err, "read pub key")
	}

	pubKey := strings.TrimSpace(string(pubKeyContent))

	tmpDir, err := os.MkdirTemp("", "cloud-init-*")
	if err != nil {
		return "", errors.Wrap(err, "create temp dir")
	}

	defer removeWithoutErrCheck(tmpDir)

	metaData := fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", instanceID, hostname)

	err = os.WriteFile(filepath.Join(tmpDir, "meta-data"), []byte(metaData), pubFilePerm)
	if err != nil {
		return "", errors.Wrap(err, "write meta-data")
	}

	userData := buildCloudInitUserData(pubKey, withNix, shutdown)

	err = os.WriteFile(filepath.Join(tmpDir, "user-data"), []byte(userData), pubFilePerm)
	if err != nil {
		return "", errors.Wrap(err, "write user-data")
	}

	out, err := exec.CommandContext(context.Background(), "genisoimage", //nolint:gosec
		"-output", seedPath, "-V", "cidata", "-r", "-J",
		filepath.Join(tmpDir, "meta-data"), filepath.Join(tmpDir, "user-data")).CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "create cloud-init seed: %s", string(out))
	}

	if !withNix && !shutdown {
		err = os.WriteFile(seedPath+".key", []byte(pubKey), pubFilePerm) //nolint:gosec
		if err != nil {
			return "", errors.Wrap(err, "write seed key cache")
		}
	}

	return seedPath, nil
}

func buildCloudInitUserData(pubKey string, withNix bool, shutdown bool) string {
	userData := "#cloud-config\ndisable_root: false\n"

	packages := []string{"rsync"}
	if withNix {
		packages = append(packages, "curl", "ca-certificates")
	}

	userData += "package_update: true\npackages:\n"

	var userDataSb110 strings.Builder
	for _, pkg := range packages {
		fmt.Fprintf(&userDataSb110, "  - %s\n", pkg)
	}

	userData += userDataSb110.String()

	userData += fmt.Sprintf(
		"write_files:\n"+
			"  - path: /root/.ssh/authorized_keys\n"+
			"    content: '%s'\n"+
			"    owner: root:root\n"+
			"    permissions: '0600'\n"+
			"    defer: true\n"+
			"runcmd:\n"+
			"  - mkdir -p /root/.ssh\n"+
			"  - chmod 700 /root/.ssh\n"+
			"  - sed -i 's/^#*PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config\n"+
			"  - mkdir -p /etc/ssh/sshd_config.d && echo 'PermitRootLogin yes' > /etc/ssh/sshd_config.d/root-login.conf\n"+
			"  - systemctl restart sshd\n",
		pubKey,
	)

	if withNix {
		// Install nix (Determinate Systems installer).
		// Used during image baking — the baked image is reused across test runs.
		// The panix SSH executioner sources /etc/profile.d/nix.sh before commands,
		// so no symlink hackery is needed.
		userData += "  - curl -sSfL https://install.determinate.systems/nix | sh -s -- install --no-confirm\n"
	}

	if shutdown {
		userData += "  - shutdown -h now\n"
	}

	return userData
}
