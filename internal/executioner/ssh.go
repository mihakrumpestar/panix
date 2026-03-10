package executioner

func (ex *Executioner) sshStream(description, statusIfRunning, statusIfFailed string, commandWithArgs []string, excOpt *ExecOptions) error {
	ssh := ex.machine.MetaInspect.GetActiveSSH()

	sshCommandWithArgs := []string{"ssh", "-q"} // Silence banners
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.MaybeSSHCommandArguments()...)
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.Hostname)

	sshCommandWithArgs = append(sshCommandWithArgs, commandWithArgs...)

	return ex.shellStream(description, statusIfRunning, statusIfFailed, sshCommandWithArgs, excOpt)
}
