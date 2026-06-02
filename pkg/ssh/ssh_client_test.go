package ssh

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestKnownHostsFile_IsAuto(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)
	assertion.False(KnownHostsFile("").IsAuto())
	assertion.True(KnownHostsFile(os.TempDir() + "/panix-knownhosts-1234").IsAuto())
	assertion.False(KnownHostsFile("/home/user/.ssh/known_hosts").IsAuto())
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
	assertion.Equal([]string{"NIX_SSHOPTS=-o StrictHostKeyChecking=accept-new"}, opts)
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
	assertion.Equal([]string{"NIX_SSHOPTS=-o IdentitiesOnly=yes -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"}, opts)
}

func TestMaybeNixSSHOpts_DisableAutoAddHostKey(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:              "example.com",
		DisableAutoAddHostKey: true,
	}

	opts := client.MaybeNixSSHOpts()

	assertion := assert.New(t)
	assertion.Equal([]string{"NIX_SSHOPTS="}, opts)
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
	assertion.Equal("ssh-ng://root@192.168.1.50", client.NixStoreURL())
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
	assertion.Equal("ssh-ng://deploy@192.168.1.50", client.NixStoreURL())
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
	assertion.Equal("ssh-ng://root@192.168.1.50?ssh-key=/home/user/.ssh/id_ed25519", client.NixStoreURL())
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
	assertion.Equal("ssh-ng://root@192.168.1.50?ssh-key=/home/user/.ssh/id_ed25519", client.NixStoreURLWithParams())
	assertion.Equal("ssh-ng://root@192.168.1.50?ssh-key=/home/user/.ssh/id_ed25519&remote-store=local?root=/mnt", client.NixStoreURLWithParams("remote-store=local?root=/mnt")) //nolint:lll
}
