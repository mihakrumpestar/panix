package ssh

import (
	"os"
	"sync"

	"github.com/pkg/errors"
)

// KnownHostsFile is the path to a known_hosts file used for SSH host key
// verification.
//
// An empty value tells panix to create a temporary file that it owns and
// removes after the workflow. A non-empty value is user-managed and is never
// removed by panix.
type KnownHostsFile string //nolint:recvcheck // Create and RemoveIfTemporary mutate; IsTemporary is read-only.

// knownHostsRegistry records the exact temporary paths panix created. Exact
// paths are used instead of a temp-directory prefix check so a user-supplied
// file under a temp-like path is never deleted.
type knownHostsRegistry struct {
	mu    sync.Mutex
	paths map[string]struct{}
}

// autoKnownHostsFiles is package-level because KnownHostsFile is a string alias
// and cannot carry ownership state itself.
var autoKnownHostsFiles = &knownHostsRegistry{paths: make(map[string]struct{})} //nolint:gochecknoglobals // package-level ownership registry

func (r *knownHostsRegistry) add(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.paths[path] = struct{}{}
}

func (r *knownHostsRegistry) owns(path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.paths[path]

	return ok
}

// forget removes path from the registry and reports whether panix owned it.
func (r *knownHostsRegistry) forget(path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.paths[path]; !ok {
		return false
	}

	delete(r.paths, path)

	return true
}

// Create creates a temporary known_hosts file when the value is empty. A
// user-provided path is left untouched.
func (k *KnownHostsFile) Create() error {
	if k != nil && *k != "" {
		return nil
	}

	tmpFile, err := os.CreateTemp("", "panix-knownhosts-")
	if err != nil {
		return errors.Wrap(err, "failed to create temp known_hosts file")
	}

	err = tmpFile.Close()
	if err != nil {
		_ = os.Remove(tmpFile.Name())

		return errors.Wrap(err, "failed to close temp known_hosts file")
	}

	*k = KnownHostsFile(tmpFile.Name())
	autoKnownHostsFiles.add(tmpFile.Name())

	return nil
}

// IsTemporary reports whether panix created this exact file. It never infers
// ownership from the path, so user-supplied files are always preserved.
func (k KnownHostsFile) IsTemporary() bool {
	if k == "" {
		return false
	}

	return autoKnownHostsFiles.owns(string(k))
}

// RemoveIfTemporary removes the file only when panix created it, then clears
// the value so a later Create can create and record a fresh temporary file.
func (k *KnownHostsFile) RemoveIfTemporary() {
	if k == nil || *k == "" {
		return
	}

	path := string(*k)
	if !autoKnownHostsFiles.forget(path) {
		return
	}

	_ = os.Remove(path)

	*k = ""
}
