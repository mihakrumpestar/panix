package executioner

func (ex *Executioner) sshStream(description, statusIfRunning, statusIfFailed string, commandWithArgs []string, excOpt *ExecOptions) error {
	ssh := ex.machine.SSH

	sshCommandWithArgs := []string{"ssh", "-q", "-t"} // Silence banners, make interactive
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.MaybeSshCommandArguments()...)
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.Hostname)

	sshCommandWithArgs = append(sshCommandWithArgs, commandWithArgs...)

	return ex.shellStream(description, statusIfRunning, statusIfFailed, sshCommandWithArgs, excOpt)
}
