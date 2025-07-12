package executioner

func (ex *Executioner) Ssh(name string, args ...string) (ExecutionerOutput, error) {

	sshArgs := []string{}

	if ex.ssh.Alias != "" {
		sshArgs = append(sshArgs, ex.ssh.Alias)
	}

	// TODO: implement more than alias

	sshArgs = append(sshArgs, name)
	sshArgs = append(sshArgs, args...)

	return ex.Shell("ssh", sshArgs...)
}
