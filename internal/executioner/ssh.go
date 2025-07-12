package executioner

func (ex *Executioner) ssh(name string, args ...string) (ExecutionerOutput, error) {

	sshArgs := []string{}

	//fmt.Printf("\nsshConfig: %+v\n\n", ex.sshConfig)

	if ex.sshConfig.Alias != "" {
		sshArgs = append(sshArgs, ex.sshConfig.Alias)
	}

	// TODO: implement more than alias

	sshArgs = append(sshArgs, name)
	sshArgs = append(sshArgs, args...)

	return ex.shell("ssh", sshArgs...)
}
