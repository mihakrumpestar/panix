package executioner

func (ex *Executioner) sshStream(description, statusIfRunning, statusIfFailed string, commandWithArgs []string, excOpt *ExecOptions) error {
	ssh := ex.conf.Machine.GetActiveSSH()

	sshCommandWithArgs := []string{"ssh", "-o", "LogLevel=ERROR"}
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.MaybeSSHCommandArguments()...)
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.SSHTarget())

	sshCommandWithArgs = append(sshCommandWithArgs, commandWithArgs...)

	return ex.shellStream(description, statusIfRunning, statusIfFailed, sshCommandWithArgs, excOpt)
}
