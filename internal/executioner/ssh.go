package executioner

import (
	"github.com/mihakrumpestar/panix/internal/pkg/logs"
)

func (ex *Executioner) sshStream(log bool, onFailure func(*logs.CommandLog, error) error, onSuccess func(*logs.CommandLog) error, commandWithArgs ...string) error {
	ssh := ex.machine.Attributes.Ssh

	sshCommandWithArgs := []string{"ssh", "-q"} // Silance banners

	if ssh.HostnameIsAlias {
		sshCommandWithArgs = append(sshCommandWithArgs, ssh.Hostname)
	} else {
		sshCommandWithArgs = append(sshCommandWithArgs, ssh.Url())

		if ex.machine.Attributes.Ssh.IdentityFile != "" {
			sshCommandWithArgs = append(sshCommandWithArgs, "-i", ssh.IdentityFile)
		}
	}

	sshCommandWithArgs = append(sshCommandWithArgs, commandWithArgs...)

	return ex.shellStream(log, onFailure, onSuccess, sshCommandWithArgs...)
}
