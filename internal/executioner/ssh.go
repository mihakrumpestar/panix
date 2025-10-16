package executioner

func (ex *Executioner) sshStream(commandWithArgs []string, excOpt *ExecOptions) error {
	ssh := ex.machine.Ssh

	sshCommandWithArgs := []string{"ssh", "-q"} // Silance banners
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.MaybeSshCommandArguments()...)
	sshCommandWithArgs = append(sshCommandWithArgs, ssh.Hostname)

	sshCommandWithArgs = append(sshCommandWithArgs, commandWithArgs...)

	return ex.shellStream(sshCommandWithArgs, excOpt)
}
