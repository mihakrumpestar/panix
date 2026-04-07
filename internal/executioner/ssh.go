package executioner

func (ex *Executioner) sshStream(description, statusIfRunning, statusIfFailed string, commandWithArgs []string, excOpt *ExecOptions) error {
	ssh := ex.machine.MetaInspect.GetActiveSSH()

	sshCommandWithArgs := []string{"ssh", "-q"} //nolint:prealloc // Silence banners, we don't need to prealloc for this one
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.MaybeSSHCommandArguments()...)
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.Hostname)

	sshCommandWithArgs = append(sshCommandWithArgs, commandWithArgs...)

	return ex.shellStream(description, statusIfRunning, statusIfFailed, sshCommandWithArgs, excOpt)
}
