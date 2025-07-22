package executioner

import "github.com/mihakrumpestar/panix/internal/config"

func (ex *Executioner) sshStream(onFailure func(*config.Log, error) error, onSuccess func(*config.Log) error, name string, args ...string) error {

	sshArgs := []string{"-q"} // Silance banners

	if ex.usesAlias {
		sshArgs = append(sshArgs, ex.machineName.Hostname())
	} else {
		// TODO: implement more than alias
	}

	sshArgs = append(sshArgs, name)
	sshArgs = append(sshArgs, args...)

	return ex.shellStream(onFailure, onSuccess, "ssh", sshArgs...)
}
