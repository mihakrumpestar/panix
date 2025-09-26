package workflow

import (
	"fmt"
	"net/url"

	"github.com/mihakrumpestar/panix/internal/config"
)

func (w *Workflow) executeBootstrapPhaseMachine(flakeName, configName string, machineName *url.URL, machine *config.Machine) error {
	// TODO: Implement nixos-anywhere bootstrap
	if w.state.Conf.Global.Verbose {
		fmt.Printf("Bootstrap for machine %s/%s/%s: TODO - implement nixos-anywhere\n", flakeName, configName, machineName)
	}

	"sudo nixos-generate-config --show-hardware-config --no-filesystems"


	//"x86_64 | aarch64)  kexecUrl="https://github.com/nix-community/nixos-images/releases/download/nixos-25.05/nixos-kexec-installer-noninteractive-${isArch}-linux.tar.gz"
	//"TMPDIR=/root/kexec setsid --wait ${maybeSudo} /root/kexec/kexec/run"
	// ssh into kexec

	// copy disko or build it on remote and run it

	// upload system closure or build it on remote

	"nixos-install --no-root-passwd --no-channel-copy --system "$nixosSystem"" // nixosSystem is path to the copied/build closure
	"reboot"
	
	return nil
}
