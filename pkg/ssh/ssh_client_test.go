package ssh

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/pkg/nixver"
)

func TestMaybeSSHCommandArguments_DisableStrictKeyChecking(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:                 "example.com",
		Port:                     22,
		Username:                 "root",
		DisableStrictKeyChecking: true,
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-o")
	assertion.Contains(args, "UserKnownHostsFile=/dev/null")
	assertion.Contains(args, "StrictHostKeyChecking=no")
}

func TestMaybeSSHCommandArguments_Defaults(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname: "example.com",
		Port:     22,
		Username: "root",
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-o")
	assertion.Contains(args, "StrictHostKeyChecking=accept-new")
	assertion.NotContains(args, "UserKnownHostsFile")
}

func TestMaybeSSHCommandArguments_KnownHostsFileWithAcceptNew(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:       "example.com",
		Port:           22,
		Username:       "root",
		KnownHostsFile: KnownHostsFile("/tmp/panix-knownhosts-test"),
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-o")
	assertion.Contains(args, "UserKnownHostsFile=/tmp/panix-knownhosts-test")
	assertion.Contains(args, "StrictHostKeyChecking=accept-new")
}

func TestMaybeSSHCommandArguments_KnownHostsFileStrictCheck(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:              "example.com",
		Port:                  22,
		Username:              "root",
		DisableAutoAddHostKey: true,
		KnownHostsFile:        KnownHostsFile("/tmp/panix-knownhosts-test"),
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-o")
	assertion.Contains(args, "UserKnownHostsFile=/tmp/panix-knownhosts-test")
	assertion.NotContains(args, "StrictHostKeyChecking")
}

func TestMaybeSSHCommandArguments_DisableStrictOverridesKnownHostsFile(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:                 "example.com",
		Port:                     22,
		Username:                 "root",
		DisableStrictKeyChecking: true,
		KnownHostsFile:           KnownHostsFile("/tmp/panix-knownhosts-test"),
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-o")
	assertion.Contains(args, "UserKnownHostsFile=/dev/null")
	assertion.Contains(args, "StrictHostKeyChecking=no")
	assertion.NotContains(args, "/tmp/panix-knownhosts-test")
}

func TestMaybeSSHCommandArguments_DisableAutoAddHostKeyWithoutKnownHostsFile(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:              "example.com",
		Port:                  22,
		Username:              "root",
		DisableAutoAddHostKey: true,
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.NotContains(args, "StrictHostKeyChecking")
	assertion.NotContains(args, "UserKnownHostsFile")
}

func TestMaybeSSHCommandArguments_AliasWithIdentityFileOmitsIdentitiesOnly(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "my-alias",
		Port:            22,
		Username:        "root",
		IdentityFile:    "/home/user/.ssh/id_ed25519",
		hostnameIsAlias: true,
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.NotContains(args, "IdentitiesOnly=yes")
	assertion.NotContains(args, "-i")
}

func TestMaybeNixSSHOpts_AliasWithIdentityFileOmitsIdentitiesOnly(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "my-alias",
		Port:            22,
		Username:        "root",
		IdentityFile:    "/home/user/.ssh/id_ed25519",
		hostnameIsAlias: true,
	}

	opts := strings.Join(client.MaybeNixSSHOpts(), " ")

	assert.NotContains(t, opts, "IdentitiesOnly=yes")
}

func TestKnownHostsFile_IsTemporary(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)
	assertion.False(KnownHostsFile("").IsTemporary())
	assertion.False(KnownHostsFile(os.TempDir()+"/panix-knownhosts-1234").IsTemporary(),
		"a path that panix did not create must never be considered auto-owned")
	assertion.False(KnownHostsFile("/home/user/.ssh/known_hosts").IsTemporary())
}

//nolint:paralleltest // t.Setenv forbids t.Parallel
func TestControlSocketPath_PrivatePerUser(t *testing.T) {
	cacheRoot := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheRoot)
	t.Setenv("HOME", t.TempDir())

	client := SSHClient{Hostname: "example.com", Port: 22, Username: "root"}

	socketPath, ok := client.controlSocketPath()
	require.True(t, ok)

	dir := filepath.Dir(socketPath)

	t.Cleanup(func() {
		// The per-user directory lives in a t.TempDir and is removed with it.
		// Remove a fallback directory too if the private cache path was
		// unavailable for any reason.
		if !strings.HasPrefix(dir, cacheRoot+string(os.PathSeparator)) {
			_ = os.RemoveAll(dir)
		}
	})

	assertion := assert.New(t)
	assertion.Contains(socketPath, "%C", "control path must use OpenSSH's connection hash token")
	assertion.True(strings.HasSuffix(dir, filepath.Join("panix", "ssh")),
		"control socket must live in panix's private cache directory")
	assertion.True(strings.HasPrefix(dir, cacheRoot+string(os.PathSeparator)),
		"control socket must live under the test cache home, not the developer home")

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assertion.Equal(os.FileMode(0o700), info.Mode().Perm())
}

// resetFallbackControlSocketDir gives each test a fresh fallback directory and
// restores the process-wide one afterwards.
func resetFallbackControlSocketDir(t *testing.T) {
	t.Helper()

	previous := fallbackControlSocketDir
	fallbackControlSocketDir = sync.OnceValues(createFallbackControlSocketDir)

	t.Cleanup(func() {
		fallbackControlSocketDir = previous
	})
}

// prepareUnsecurableCacheDir makes the per-user control socket directory
// impossible to create by placing a regular file where the "panix" directory
// would be.
func prepareUnsecurableCacheDir(t *testing.T) string {
	t.Helper()

	cacheRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(cacheRoot, "panix"), []byte("not a directory"), 0o600))
	t.Setenv("XDG_CACHE_HOME", cacheRoot)

	return cacheRoot
}

//nolint:paralleltest // t.Setenv forbids t.Parallel
func TestControlSocketPath_FallsBackToPrivateTempDir(t *testing.T) {
	resetFallbackControlSocketDir(t)

	cacheRoot := prepareUnsecurableCacheDir(t)
	client := SSHClient{Hostname: "example.com", Port: 22, Username: "root"}

	socketPath, ok := client.controlSocketPath()
	require.True(t, ok)

	dir := filepath.Dir(socketPath)

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	assertion := assert.New(t)
	assertion.True(strings.HasSuffix(socketPath, "%C"), "fallback must keep the %C token")
	assertion.True(strings.HasPrefix(filepath.Base(dir), "panix-ssh-"),
		"fallback must use a private MkdirTemp directory")
	assertion.NotEqual(cacheRoot, dir)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assertion.Equal(os.FileMode(0o700), info.Mode().Perm(), "fallback directory must be private")
}

//nolint:paralleltest // t.Setenv forbids t.Parallel
func TestControlMasterArgs_DisabledWhenNoPrivateDirAvailable(t *testing.T) {
	resetFallbackControlSocketDir(t)

	prepareUnsecurableCacheDir(t)

	// Point TMPDIR at a missing directory so MkdirTemp fails as well.
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))

	client := SSHClient{Hostname: "example.com", Port: 22, Username: "root"}

	assert.Empty(t, client.controlMasterArgs(), "ControlMaster must be disabled rather than using an insecure path")

	socketPath, ok := client.controlSocketPath()
	assert.False(t, ok)
	assert.Empty(t, socketPath)
}

func TestMaybeSSHCommandArguments_ExtraFlagsPrecedePanixOptions(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:   "example.com",
		Port:       22,
		Username:   "root",
		ExtraFlags: []string{"-o", "StrictHostKeyChecking=yes"},
	}

	args := client.MaybeSSHCommandArguments()

	userIdx := slices.Index(args, "StrictHostKeyChecking=yes")
	panixIdx := slices.Index(args, "StrictHostKeyChecking=accept-new")

	require.NotEqual(t, -1, userIdx, "user flag must be present")
	require.NotEqual(t, -1, panixIdx, "panix default must be present")
	assert.Less(t, userIdx, panixIdx, "user flag must come first so OpenSSH honours it")
}

func TestMaybeNixSSHOpts_ExtraFlagsPrecedePanixOptions(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:   "example.com",
		Port:       22,
		Username:   "root",
		ExtraFlags: []string{"-o", "StrictHostKeyChecking=yes"},
	}

	opts := strings.Join(client.MaybeNixSSHOpts(), " ")

	userIdx := strings.Index(opts, "StrictHostKeyChecking=yes")
	panixIdx := strings.Index(opts, "StrictHostKeyChecking=accept-new")

	require.NotEqual(t, -1, userIdx, "user flag must be present in NIX_SSHOPTS")
	require.NotEqual(t, -1, panixIdx, "panix default must be present in NIX_SSHOPTS")
	assert.Less(t, userIdx, panixIdx, "user flag must come first in NIX_SSHOPTS")
}

func TestMaybeNixSSHOpts_Defaults(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname: "example.com",
		Port:     22,
		Username: "root",
	}

	opts := client.MaybeNixSSHOpts()

	assertion := assert.New(t)
	assertion.Contains(opts[0], "StrictHostKeyChecking=accept-new")
	assertion.Contains(opts[0], "ControlMaster=auto")
	assertion.Contains(opts[0], "ControlPersist=60")
}

func TestMaybeNixSSHOpts_DisableStrictKeyChecking(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:                 "example.com",
		DisableStrictKeyChecking: true,
		IdentityFile:             "/home/user/.ssh/id_ed25519",
	}

	opts := client.MaybeNixSSHOpts()

	assertion := assert.New(t)
	assertion.Contains(opts[0], "IdentitiesOnly=yes")
	assertion.Contains(opts[0], "UserKnownHostsFile=/dev/null")
	assertion.Contains(opts[0], "StrictHostKeyChecking=no")
	assertion.Contains(opts[0], "ControlMaster=auto")
	assertion.Contains(opts[0], "ControlPersist=60")
}

func TestMaybeNixSSHOpts_DisableAutoAddHostKey(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:              "example.com",
		DisableAutoAddHostKey: true,
	}

	opts := client.MaybeNixSSHOpts()

	assertion := assert.New(t)
	assertion.Contains(opts[0], "ControlMaster=auto")
	assertion.NotContains(opts[0], "StrictHostKeyChecking")
	assertion.NotContains(opts[0], "UserKnownHostsFile")
}

func TestNixStoreURL_Alias(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "my-server",
		hostnameIsAlias: true,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://my-server", client.NixStoreURL())
}

func TestNixStoreURL_AliasWithResolvedIP(t *testing.T) {
	t.Parallel()

	// After SSH config resolution: Hostname=IP, alias=machine name
	client := SSHClient{
		Hostname:        "10.0.0.1",
		Port:            22,
		hostnameIsAlias: true,
		alias:           "my-server",
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://my-server", client.NixStoreURL(), "should use alias not resolved IP")
	assertion.Equal("10.0.0.1:22", client.HostPortString(), "HostPortString should use resolved IP")
	assertion.Equal("my-server", client.SSHTarget(), "SSHTarget should return alias")
}

func TestNixStoreURL_NonAlias_Defaults(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "192.168.1.50",
		Username:        SSHDefaultUsername,
		Port:            SSHDefaultPort,
		hostnameIsAlias: false,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://root@192.168.1.50:22", client.NixStoreURL())
}

func TestNixStoreURL_NonAlias_CustomUser(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "192.168.1.50",
		Username:        "deploy",
		Port:            SSHDefaultPort,
		hostnameIsAlias: false,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://deploy@192.168.1.50:22", client.NixStoreURL())
}

func TestNixStoreURL_NonAlias_CustomPort(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "192.168.1.50",
		Port:            2222,
		Username:        "root",
		hostnameIsAlias: false,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://root@192.168.1.50:2222", client.NixStoreURL())
}

func TestNixStoreURL_NonAlias_IdentityFile(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "192.168.1.50",
		Username:        SSHDefaultUsername,
		Port:            SSHDefaultPort,
		IdentityFile:    "/home/user/.ssh/id_ed25519",
		hostnameIsAlias: false,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://root@192.168.1.50:22?ssh-key=/home/user/.ssh/id_ed25519", client.NixStoreURL())
}

func TestNixStoreURL_NonAlias_AllCustom(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "192.168.1.50",
		Port:            2222,
		Username:        "builder",
		IdentityFile:    "/home/user/.ssh/builder_key",
		hostnameIsAlias: false,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://builder@192.168.1.50:2222?ssh-key=/home/user/.ssh/builder_key", client.NixStoreURL())
}

func TestResolveIdentityFile_Empty(t *testing.T) {
	t.Parallel()

	result, err := resolveIdentityFile("")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestResolveIdentityFile_AbsolutePath(t *testing.T) {
	t.Parallel()

	result, err := resolveIdentityFile("/home/user/.ssh/id_ed25519")
	require.NoError(t, err)
	assert.Equal(t, "/home/user/.ssh/id_ed25519", result)
}

func TestResolveIdentityFile_TildeExpansion(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	assertion := assert.New(t)
	result, err := resolveIdentityFile("~/.ssh/id_ed25519")
	require.NoError(t, err)
	assertion.Equal(home+"/.ssh/id_ed25519", result)
}

func TestSSHTarget_NonAlias(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname: "192.168.1.50",
	}

	assertion := assert.New(t)
	assertion.Equal("192.168.1.50", client.SSHTarget(), "non-alias should return Hostname")
}

func TestSSHTarget_Alias(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "10.0.0.1",
		hostnameIsAlias: true,
		alias:           "my-server",
	}

	assertion := assert.New(t)
	assertion.Equal("my-server", client.SSHTarget(), "alias should return alias name, not resolved IP")
}

func TestSSHTarget_NoAliasSet(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "my-server",
		hostnameIsAlias: true,
	}

	assertion := assert.New(t)
	assertion.Equal("my-server", client.SSHTarget(), "should fall back to Hostname when alias is empty")
}

func TestNixStoreURLWithParams_Alias(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "my-server",
		hostnameIsAlias: true,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://my-server", client.NixStoreURLWithParams())
	assertion.Equal("ssh-ng://my-server?remote-store=local?root=/mnt", client.NixStoreURLWithParams("remote-store=local?root=/mnt"))
}

func TestNixStoreURLWithParams_NonAliasWithIdentityFile(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "192.168.1.50",
		Username:        SSHDefaultUsername,
		Port:            SSHDefaultPort,
		IdentityFile:    "/home/user/.ssh/id_ed25519",
		hostnameIsAlias: false,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://root@192.168.1.50:22?ssh-key=/home/user/.ssh/id_ed25519", client.NixStoreURLWithParams())
	assertion.Equal("ssh-ng://root@192.168.1.50:22?ssh-key=/home/user/.ssh/id_ed25519&remote-store=local?root=/mnt", client.NixStoreURLWithParams("remote-store=local?root=/mnt")) //nolint:lll
}

var lixInfo = nixver.ParseNixVersion("nix (Lix, like Nix) 2.94.0")

func TestNixStoreURL_NonAlias_LixDefaults(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "192.168.1.50",
		Username:        SSHDefaultUsername,
		Port:            SSHDefaultPort,
		hostnameIsAlias: false,
		nixInfo:         *lixInfo,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://root@192.168.1.50?port=22", client.NixStoreURL())
}

func TestNixStoreURL_NonAlias_LixCustomPort(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "192.168.1.50",
		Port:            2222,
		Username:        "root",
		hostnameIsAlias: false,
		nixInfo:         *lixInfo,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://root@192.168.1.50?port=2222", client.NixStoreURL())
}

func TestNixStoreURL_NonAlias_LixIdentityFile(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "192.168.1.50",
		Username:        SSHDefaultUsername,
		Port:            SSHDefaultPort,
		IdentityFile:    "/home/user/.ssh/id_ed25519",
		hostnameIsAlias: false,
		nixInfo:         *lixInfo,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://root@192.168.1.50?port=22&ssh-key=/home/user/.ssh/id_ed25519", client.NixStoreURL())
}

func TestNixStoreURL_NonAlias_LixAllCustom(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "192.168.1.50",
		Port:            2222,
		Username:        "builder",
		IdentityFile:    "/home/user/.ssh/builder_key",
		hostnameIsAlias: false,
		nixInfo:         *lixInfo,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://builder@192.168.1.50?port=2222&ssh-key=/home/user/.ssh/builder_key", client.NixStoreURL())
}

func TestNixStoreURLWithParams_NonAlias_LixWithIdentityFile(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "192.168.1.50",
		Username:        SSHDefaultUsername,
		Port:            SSHDefaultPort,
		IdentityFile:    "/home/user/.ssh/id_ed25519",
		hostnameIsAlias: false,
		nixInfo:         *lixInfo,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://root@192.168.1.50?port=22&ssh-key=/home/user/.ssh/id_ed25519", client.NixStoreURLWithParams())
	assertion.Equal("ssh-ng://root@192.168.1.50?port=22&ssh-key=/home/user/.ssh/id_ed25519&remote-store=local?root=/mnt", client.NixStoreURLWithParams("remote-store=local?root=/mnt")) //nolint:lll
}

func TestNixStoreURL_NonAlias_LixAlias(t *testing.T) {
	t.Parallel()

	// Alias mode should not be affected by Lix detection.
	client := SSHClient{
		Hostname:        "my-alias",
		hostnameIsAlias: true,
	}

	assertion := assert.New(t)
	assertion.Equal("ssh-ng://my-alias", client.NixStoreURL())
}
