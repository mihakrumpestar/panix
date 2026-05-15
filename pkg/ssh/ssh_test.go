package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinburke/ssh_config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveHostname_AllFieldsUnset(t *testing.T) {
	t.Parallel()

	client := &SSHClient{}
	client.resolveHostname("my-alias")

	assert.True(t, client.hostnameIsAlias)
	assert.Equal(t, "my-alias", client.Hostname)
}

func TestResolveHostname_HostnameSet(t *testing.T) {
	t.Parallel()

	client := &SSHClient{Hostname: "explicit.host"}
	client.resolveHostname("machine-name")

	assert.False(t, client.hostnameIsAlias)
	assert.Equal(t, "explicit.host", client.Hostname)
}

func TestResolveHostname_PortSet(t *testing.T) {
	t.Parallel()

	client := &SSHClient{Port: 2222}
	client.resolveHostname("machine-name")

	assert.False(t, client.hostnameIsAlias)
	assert.Equal(t, "machine-name", client.Hostname)
}

func TestResolveHostname_UsernameSet(t *testing.T) {
	t.Parallel()

	client := &SSHClient{Username: "deploy"}
	client.resolveHostname("machine-name")

	assert.False(t, client.hostnameIsAlias)
	assert.Equal(t, "machine-name", client.Hostname)
}

func TestResolveHostname_IdentityFileSet(t *testing.T) {
	t.Parallel()

	client := &SSHClient{IdentityFile: "/home/user/.ssh/key"}
	client.resolveHostname("machine-name")

	assert.False(t, client.hostnameIsAlias)
	assert.Equal(t, "machine-name", client.Hostname)
}

func TestApplyDefaults(t *testing.T) {
	t.Parallel()

	client := &SSHClient{}
	client.applyDefaults()

	assert.Equal(t, SSHDefaultUsername, client.Username)
	assert.Equal(t, uint16(SSHDefaultPort), client.Port)
}

func TestApplyDefaults_PreservesExisting(t *testing.T) {
	t.Parallel()

	client := &SSHClient{Username: "deploy", Port: 2222}
	client.applyDefaults()

	assert.Equal(t, "deploy", client.Username)
	assert.Equal(t, uint16(2222), client.Port)
}

func TestIsInitialized(t *testing.T) {
	t.Parallel()

	empty := SSHClient{}
	assert.False(t, empty.IsInitialized())

	set := SSHClient{Hostname: "example.com"}
	assert.True(t, set.IsInitialized())
}

func TestIsLocal(t *testing.T) {
	t.Parallel()

	client := SSHClient{Hostname: "localhost", isLocal: true}
	assert.True(t, client.IsLocal())

	remote := SSHClient{Hostname: "remote.host", isLocal: false}
	assert.False(t, remote.IsLocal())
}

func TestPortString(t *testing.T) {
	t.Parallel()

	client := SSHClient{Port: 22}
	assert.Equal(t, "22", client.PortString())

	custom := SSHClient{Port: 2222}
	assert.Equal(t, "2222", custom.PortString())
}

func TestHostPortString(t *testing.T) {
	t.Parallel()

	client := SSHClient{Hostname: "example.com", Port: 22}
	assert.Equal(t, "example.com:22", client.HostPortString())

	custom := SSHClient{Hostname: "192.168.1.1", Port: 2222}
	assert.Equal(t, "192.168.1.1:2222", custom.HostPortString())
}

func TestMaybeSSHCommandArguments_WithIdentityFile(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:     "example.com",
		Port:         22,
		Username:     "root",
		IdentityFile: "/home/user/.ssh/id_ed25519",
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-i")
	assertion.Contains(args, "/home/user/.ssh/id_ed25519")
	assertion.Contains(args, "-o")
	assertion.Contains(args, "IdentitiesOnly=yes")
}

func TestMaybeSSHCommandArguments_Alias(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:        "my-alias",
		Port:            22,
		Username:        "root",
		hostnameIsAlias: true,
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.NotContains(args, "-l")
	assertion.NotContains(args, "-p")
	assertion.Contains(args, "-o")
	assertion.Contains(args, "StrictHostKeyChecking=accept-new")
}

func TestMaybeSSHCommandArguments_ExtraFlags(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:   "example.com",
		Port:       22,
		Username:   "root",
		ExtraFlags: []string{"-v", "-o", "Compression=yes"},
	}

	args := client.MaybeSSHCommandArguments()

	assertion := assert.New(t)
	assertion.Contains(args, "-v")
	assertion.Contains(args, "Compression=yes")
}

func TestInit_ResolveHostnameAlias(t *testing.T) {
	t.Parallel()

	cfgStr := `Host my-alias
    HostName 10.0.0.1
    User deploy
    Port 2222
`
	cfg, err := ssh_config.Decode(strings.NewReader(cfgStr))
	require.NoError(t, err)

	sshCfg := &SSHConfig{sshConfig: cfg}
	client := &SSHClient{}

	err = client.Init(sshCfg, "my-alias", "localhost")
	require.NoError(t, err)

	assert.True(t, client.hostnameIsAlias)
	assert.Equal(t, "10.0.0.1", client.Hostname)
	assert.Equal(t, "deploy", client.Username)
	assert.Equal(t, uint16(2222), client.Port)
}

func TestInit_ExplicitHostname(t *testing.T) {
	t.Parallel()

	cfg := &SSHConfig{}
	client := &SSHClient{Hostname: "192.168.1.1", Port: 2222, Username: "deploy"}

	err := client.Init(cfg, "machine-name", "localhost")
	require.NoError(t, err)

	assert.False(t, client.hostnameIsAlias)
	assert.Equal(t, "192.168.1.1", client.Hostname)
	assert.Equal(t, "deploy", client.Username)
	assert.Equal(t, uint16(2222), client.Port)
}

func TestInit_IsLocal(t *testing.T) {
	t.Parallel()

	cfg := &SSHConfig{}
	client := &SSHClient{Hostname: "this-machine"}

	err := client.Init(cfg, "machine-name", "this-machine")
	require.NoError(t, err)

	assert.True(t, client.IsLocal())
}

func TestInit_IsRemote(t *testing.T) {
	t.Parallel()

	cfg := &SSHConfig{}
	client := &SSHClient{Hostname: "remote-host"}

	err := client.Init(cfg, "machine-name", "this-machine")
	require.NoError(t, err)

	assert.False(t, client.IsLocal())
}

func TestKnownHostsFile_Create_Provided(t *testing.T) {
	t.Parallel()

	provided := KnownHostsFile("/home/user/.ssh/known_hosts")
	err := provided.Create()
	require.NoError(t, err)
	assert.Equal(t, KnownHostsFile("/home/user/.ssh/known_hosts"), provided)
}

func TestKnownHostsFile_Create_Auto(t *testing.T) {
	t.Parallel()

	var knownHosts KnownHostsFile

	err := knownHosts.Create()
	require.NoError(t, err)

	t.Cleanup(func() {
		knownHosts.RemoveIfAuto()
	})

	assert.True(t, knownHosts.IsAuto())

	info, statErr := os.Stat(string(knownHosts))
	require.NoError(t, statErr)
	assert.False(t, info.IsDir())
}

func TestKnownHostsFile_RemoveIfAuto_NotAuto(t *testing.T) {
	t.Parallel()

	k := KnownHostsFile("/home/user/.ssh/known_hosts")
	k.RemoveIfAuto() // should be a no-op
}

func TestKnownHostsFile_RemoveIfAuto_Empty(t *testing.T) {
	t.Parallel()

	var k KnownHostsFile
	k.RemoveIfAuto() // should be a no-op
}

func TestSSHConfig_RetrieveFullParamsFromSSHConfig(t *testing.T) {
	t.Parallel()

	cfgStr := `Host my-server
    HostName 192.168.1.100
    Port 2222
    User deploy
`
	cfg, err := ssh_config.Decode(strings.NewReader(cfgStr))
	require.NoError(t, err)

	sshCfg := &SSHConfig{sshConfig: cfg}
	client := &SSHClient{Hostname: "my-server", hostnameIsAlias: true}

	err = sshCfg.RetrieveFullParamsFromSSHConfig(client)
	require.NoError(t, err)

	assert.Equal(t, "192.168.1.100", client.Hostname)
	assert.Equal(t, uint16(2222), client.Port)
	assert.Equal(t, "deploy", client.Username)
}

func TestSSHConfig_RetrieveFullParamsFromSSHConfig_MissingHostname(t *testing.T) {
	t.Parallel()

	cfgStr := `Host my-server
    User deploy
`
	cfg, err := ssh_config.Decode(strings.NewReader(cfgStr))
	require.NoError(t, err)

	sshCfg := &SSHConfig{sshConfig: cfg}
	client := &SSHClient{Hostname: "my-server", hostnameIsAlias: true}

	err = sshCfg.RetrieveFullParamsFromSSHConfig(client)
	assert.ErrorIs(t, err, ErrSSHConfigMissingHostname)
}

func TestSSHConfig_RetrieveFullParamsFromSSHConfig_NilReceiver(t *testing.T) {
	t.Parallel()

	var sshCfg *SSHConfig

	client := &SSHClient{Hostname: "my-server"}

	err := sshCfg.RetrieveFullParamsFromSSHConfig(client)
	assert.NoError(t, err)
}

func TestSSHConfig_RetrieveFullParamsFromSSHConfig_PartialConfig(t *testing.T) {
	t.Parallel()

	cfgStr := `Host my-server
    HostName 10.0.0.1
`
	cfg, err := ssh_config.Decode(strings.NewReader(cfgStr))
	require.NoError(t, err)

	sshCfg := &SSHConfig{sshConfig: cfg}
	client := &SSHClient{Hostname: "my-server", hostnameIsAlias: true}

	err = sshCfg.RetrieveFullParamsFromSSHConfig(client)
	require.NoError(t, err)

	assert.Equal(t, "10.0.0.1", client.Hostname)
	assert.Equal(t, uint16(0), client.Port)
	assert.Empty(t, client.Username)
}

func TestResolveIdentityFile_RelativePath(t *testing.T) {
	t.Parallel()

	result, err := resolveIdentityFile("keys/id_ed25519")
	require.NoError(t, err)

	absPath, absErr := filepath.Abs("keys/id_ed25519")
	require.NoError(t, absErr)
	assert.Equal(t, absPath, result)
}

func TestMaybeNixSSHOpts_IdentityFile(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:     "example.com",
		IdentityFile: "/home/user/.ssh/id_ed25519",
	}

	opts := client.MaybeNixSSHOpts()

	assertion := assert.New(t)
	assertion.Contains(opts[0], "IdentitiesOnly=yes")
}

func TestMaybeNixSSHOpts_KnownHostsFile(t *testing.T) {
	t.Parallel()

	client := SSHClient{
		Hostname:       "example.com",
		KnownHostsFile: KnownHostsFile("/home/user/.ssh/known_hosts"),
	}

	opts := client.MaybeNixSSHOpts()

	assertion := assert.New(t)
	assertion.Contains(opts[0], "UserKnownHostsFile=/home/user/.ssh/known_hosts")
	assertion.Contains(opts[0], "StrictHostKeyChecking=accept-new")
}

func TestConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "root", SSHDefaultUsername)
	assert.Equal(t, 22, SSHDefaultPort)
}
