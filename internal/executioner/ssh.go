package executioner

func (ex *Executioner) sshStream(name string, args ...string) <-chan ExecutionerOutput {

	sshArgs := []string{"-q"} // Silance banners

	//fmt.Printf("\nsshConfig: %+v\n\n", ex.sshConfig)

	if ex.usesAlias {
		sshArgs = append(sshArgs, ex.machineName.Host)
	} else {
		// TODO: implement more than alias
	}

	sshArgs = append(sshArgs, name)
	sshArgs = append(sshArgs, args...)

	return ex.shellStream("ssh", sshArgs...)
}
