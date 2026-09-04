package phaseops

// WithEnv prefixes command with env KEY=VALUE pairs so the variables
// reach the final command across every boundary panix uses: direct exec,
// sudo env_reset, su -l login reset, and ssh remote re-parsing. As argv
// elements they are shell-agnostic. Empty env returns command unchanged.
func WithEnv(env []string, command []string) []string {
	if len(env) == 0 {
		return command
	}

	args := make([]string, 0, len(env)+len(command)+1)
	args = append(args, "env")
	args = append(args, env...)

	return append(args, command...)
}
