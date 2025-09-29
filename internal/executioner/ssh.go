package executioner

import "github.com/mihakrumpestar/panix/internal/config"

func (ex *Executioner) sshStream(log bool, onFailure func(*config.CommandLog, error) error, onSuccess func(*config.CommandLog) error, commandWithArgs ...string) error {

	sshCommandWithArgs := []string{"ssh", "-q"} // Silance banners

	if ex.machine.Ssh.HostnameIsAlias {
		sshCommandWithArgs = append(sshCommandWithArgs, ex.machine.Ssh.Hostname)
	} else {
		// TODO: implement more than alias
		panic("not implemented: support more than alias for SSH")
	}

	sshCommandWithArgs = append(sshCommandWithArgs, commandWithArgs...)

	return ex.shellStream(log, onFailure, onSuccess, sshCommandWithArgs...)
}
