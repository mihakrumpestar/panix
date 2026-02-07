package executioner

func (ex *Executioner) sshStream(description, statusIfFailed string, commandWithArgs []string, excOpt *ExecOptions) error {
	ssh := ex.machine.SSH

	sshCommandWithArgs := []string{"ssh", "-q", "-t"} // Silance banners, make interactive
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.MaybeSshCommandArguments()...)
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.Hostname)

	sshCommandWithArgs = append(sshCommandWithArgs, commandWithArgs...)

	return ex.shellStream(description, statusIfFailed, sshCommandWithArgs, excOpt)
}
