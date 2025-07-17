package executioner

func (ex *Executioner) sshStream(onFailure func(*BaseMeta, error) error, onSuccess func(*BaseMeta), name string, args ...string) error {

	sshArgs := []string{"-q"} // Silance banners

	//fmt.Printf("\nsshConfig: %+v\n\n", ex.sshConfig)

	if ex.usesAlias {
		sshArgs = append(sshArgs, ex.meta.MachineName.Hostname())
	} else {
		// TODO: implement more than alias
	}

	sshArgs = append(sshArgs, name)
	sshArgs = append(sshArgs, args...)

	return ex.shellStream(onFailure, onSuccess, "ssh", sshArgs...)
}
