package bootstrap

import (
	"slices"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/pkg/urlx"
	"github.com/pkg/errors"
)

// downloadArgv returns the curl argv that fetches sourceURL to remotePath on
// the target. Pure argv, no shell strings.
func downloadArgv(curlFlags []string, sourceURL, remotePath string) []string {
	return slices.Concat([]string{"curl"}, curlFlags, []string{sourceURL, "-o", remotePath})
}

// downloadOrTransfer stages source at remotePath on the target: an http(s) URL
// is downloaded with curl on the target itself, a local path is transferred
// with rsync. Any other URL scheme is rejected before any command runs. name
// labels the commands and error context.
func downloadOrTransfer(
	exc *executioner.Executioner,
	machineI *machine.Machine,
	source, remotePath, name string,
	curlFlags []string,
) error {
	err := urlx.ValidateSource(source)
	if err != nil {
		return errors.Wrap(err, name+" transfer failed")
	}

	if urlx.IsHTTPURL(source) {
		err = exc.Exec(
			"download "+name,
			"downloading "+name,
			"failed to download "+name,
			downloadArgv(curlFlags, source, remotePath),
		)
	} else {
		err = phaseops.TransferFile(exc, machineI, attributes.TransferSource{
			LocalPath:  source,
			RemotePath: remotePath,
		}, name, false)
	}

	return errors.Wrap(err, name+" transfer failed")
}
