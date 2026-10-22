package bootstrap

import (
	"crypto/rand"
	"encoding/hex"
	"path"
	"slices"
	"strings"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/executioner"
	"github.com/mihakrumpestar/panix/internal/workflow/phaseops"
	"github.com/mihakrumpestar/panix/pkg/urlx"
	"github.com/pkg/errors"
)

const (
	// stagingBaseDir is the remote parent for per-run bootstrap staging dirs.
	stagingBaseDir = "/tmp"
	// stagingDirMode keeps the per-run staging dir private to the SSH user.
	stagingDirMode = "700"
	// stagingSuffixBytes makes a per-run staging path unguessable and unique.
	stagingSuffixBytes = 16
)

// downloadArgv returns the curl argv that fetches sourceURL to remotePath on
// the target. Resume flags are stripped: each artifact is written to a fresh
// per-run path, so resuming a stale or attacker-created file is never valid.
// The protocol restrictions come last so an operator-supplied flag cannot
// re-enable plain http or a redirect downgrade from https.
func downloadArgv(curlFlags []string, sourceURL, remotePath string) []string {
	return slices.Concat(
		[]string{"curl"},
		withoutResumeFlags(curlFlags),
		[]string{"--proto", "=https", "--proto-redir", "=https", sourceURL, "-o", remotePath},
	)
}

// downloadOrTransfer stages source at remotePath on the target: an https URL
// is downloaded with curl on the target itself, a local path is transferred
// with rsync. Plain http and any other URL scheme are rejected before any
// command runs. name labels the commands and error context.
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

// newStagingDirPath returns a unique per-run staging directory under /tmp.
// The random suffix prevents a predictable-path race and stale reuse.
func newStagingDirPath(prefix string) (string, error) {
	suffix, err := randomStagingSuffix()
	if err != nil {
		return "", errors.Wrap(err, "failed to generate staging directory name")
	}

	return path.Join(stagingBaseDir, prefix+"-"+suffix), nil
}

// createPrivateStagingDir creates stagingDir with mode 0700 owned by the
// active SSH user. Elevation lets non-root SSH users own the directory they
// will write to unelevated.
func createPrivateStagingDir(exc *executioner.Executioner, machineI *machine.Machine, stagingDir string) error {
	sshUser := machineI.GetActiveSSH().Username

	err := exc.Exec(
		"create staging directory",
		"creating staging directory",
		"failed to create staging directory",
		slices.Concat(
			machineI.MaybeSudo(),
			[]string{"install", "-d", "-m", stagingDirMode, "-o", sshUser, stagingDir},
		),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create staging directory")
	}

	return nil
}

// removeStagingDir removes a per-run staging directory. Callers treat it as
// best effort: cleanup must not mask the primary failure.
func removeStagingDir(exc *executioner.Executioner, machineI *machine.Machine, stagingDir string) error {
	err := exc.Exec(
		"remove staging directory",
		"removing staging directory",
		"failed to remove staging directory",
		slices.Concat(machineI.MaybeSudo(), []string{"rm", "-rf", "--", stagingDir}),
	)
	if err != nil {
		return errors.Wrap(err, "failed to remove staging directory")
	}

	return nil
}

// withoutResumeFlags removes curl resume flags (-C, -C-, --continue-at) and
// their values from a configured flag list.
func withoutResumeFlags(flags []string) []string {
	sanitized := make([]string, 0, len(flags))

	for index := 0; index < len(flags); index++ {
		switch {
		case flags[index] == "-C" || flags[index] == "--continue-at":
			// The value belongs to the resume flag and must be dropped too.
			if index+1 < len(flags) {
				index++
			}
		case strings.HasPrefix(flags[index], "-C") && len(flags[index]) > len("-C"):
			continue
		case strings.HasPrefix(flags[index], "--continue-at="):
			continue
		default:
			sanitized = append(sanitized, flags[index])
		}
	}

	return sanitized
}

func randomStagingSuffix() (string, error) {
	randomBytes := make([]byte, stagingSuffixBytes)

	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", errors.Wrap(err, "failed to read random bytes")
	}

	return hex.EncodeToString(randomBytes), nil
}
