// Package argv assembles a command line from a prefix, global flags and flag
// groups.
package argv

// Argv assembles prefix, global flags, then each group.
//
// Global flags always follow the whole prefix. nix accepts them anywhere after
// its subcommand, and drop-in tools such as nom reject anything before theirs,
// so this order is the only one that works for both.
func Argv(prefix, globalFlags []string, groups ...[]string) []string {
	total := len(prefix) + len(globalFlags)
	for _, group := range groups {
		total += len(group)
	}

	argv := make([]string, 0, total)
	argv = append(argv, prefix...)
	argv = append(argv, globalFlags...)

	for _, group := range groups {
		argv = append(argv, group...)
	}

	return argv
}
