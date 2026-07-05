package machine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/logs"
	"github.com/mihakrumpestar/panix/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/pkg/ssh"
	"github.com/mihakrumpestar/panix/pkg/xpath"
)

// --- Helpers ---

// newTestMachine creates a Machine with State and MetaInspect initialized to
// zero values. Callers set specific fields (SSH, Bootstrap, etc.) per test.
func newTestMachine() *Machine {
	return &Machine{
		State:       atomicpointer.New[State](),
		MetaInspect: atomicpointer.New[MetaInspect](),
	}
}

// --- GetActiveSSH: SSHTypeRegular ---

func TestGetActiveSSH_Regular(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.SSH = ssh.SSHClient{
		Hostname:     "10.0.0.1",
		Port:         22,
		Username:     "root",
		IdentityFile: "/home/user/.ssh/id_ed25519",
	}
	mach.State.Store(&State{ActiveSSH: SSHTypeRegular})

	result := mach.GetActiveSSH()

	assertion := assert.New(t)
	assertion.Equal("10.0.0.1", result.Hostname)
	assertion.Equal(uint16(22), result.Port)
	assertion.Equal("root", result.Username)
	assertion.Equal("/home/user/.ssh/id_ed25519", result.IdentityFile)
}

func TestGetActiveSSH_EmptyActiveSSH_DefaultsToRegular(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.SSH = ssh.SSHClient{Hostname: "10.0.0.1", Port: 22, Username: "root"}
	// ActiveSSH is "" (zero value of SSHType) — should default to SSHTypeRegular
	mach.State.Store(&State{})

	result := mach.GetActiveSSH()

	assert.Equal(t, "10.0.0.1", result.Hostname)
}

// --- GetActiveSSH: SSHTypeBootstrap ---

func TestGetActiveSSH_Bootstrap(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.Bootstrap.SSH = ssh.SSHClient{
		Hostname:     "10.0.0.2",
		Port:         22,
		Username:     "root",
		IdentityFile: "/home/user/.ssh/bootstrap_key",
	}
	mach.State.Store(&State{ActiveSSH: SSHTypeBootstrap})

	result := mach.GetActiveSSH()

	assertion := assert.New(t)
	assertion.Equal("10.0.0.2", result.Hostname)
	assertion.Equal(uint16(22), result.Port)
	assertion.Equal("root", result.Username)
	assertion.Equal("/home/user/.ssh/bootstrap_key", result.IdentityFile)
}

// --- GetActiveSSH: SSHTypeKexec ---
//
// These tests cover the critical kexec reconnect path where the SSH port is
// derived from KexecConfig.SSHPort via Get(). The default (zero) value must
// resolve to ssh.SSHDefaultPort (22), not 0 — this was the root cause of
// panix failing to reconnect after kexec boot.

func TestGetActiveSSH_Kexec_DefaultSSHPort(t *testing.T) {
	t.Parallel()

	// Regression test: SSHPort=0 (not set in YAML) must resolve to port 22.
	// Before the fix, the raw uint16 zero value was used directly, causing
	// ReachabilityCheck to dial hostname:0 — which always fails.
	mach := newTestMachine()
	mach.SSH = ssh.SSHClient{Hostname: "10.0.0.3", Port: 22222, Username: "root"}
	mach.Bootstrap.SSH = ssh.SSHClient{
		Hostname:     "10.0.0.3",
		Port:         22,
		Username:     "root",
		IdentityFile: "/home/user/.ssh/bootstrap_key",
	}
	mach.Bootstrap.Kexec.SSHPort = 0 // not set in YAML
	mach.State.Store(&State{ActiveSSH: SSHTypeKexec})

	result := mach.GetActiveSSH()

	assertion := assert.New(t)
	assertion.Equal("10.0.0.3", result.Hostname, "hostname should come from bootstrap SSH")
	assertion.Equal(uint16(22), result.Port, "port must default to 22 when SSHPort is zero")
	assertion.Equal("root", result.Username)
	assertion.Equal("/home/user/.ssh/bootstrap_key", result.IdentityFile)
}

func TestGetActiveSSH_Kexec_CustomSSHPort(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.SSH = ssh.SSHClient{Hostname: "10.0.0.3", Port: 22222, Username: "root"}
	mach.Bootstrap.SSH = ssh.SSHClient{
		Hostname:     "10.0.0.3",
		Port:         22,
		Username:     "root",
		IdentityFile: "/home/user/.ssh/bootstrap_key",
	}
	mach.Bootstrap.Kexec.SSHPort = 22222
	mach.State.Store(&State{ActiveSSH: SSHTypeKexec})

	result := mach.GetActiveSSH()

	assertion := assert.New(t)
	assertion.Equal("10.0.0.3", result.Hostname)
	assertion.Equal(uint16(22222), result.Port, "port should be overridden to custom SSHPort")
	assertion.Equal("root", result.Username)
	assertion.Equal("/home/user/.ssh/bootstrap_key", result.IdentityFile)
}

func TestGetActiveSSH_Kexec_BootstrapSSHNotInitialized_DefaultPort(t *testing.T) {
	t.Parallel()

	// When bootstrap SSH is not initialized, the kexec SSH falls back to
	// the regular SSH config (mach.SSH) with the SSHPort override applied.
	mach := newTestMachine()
	mach.SSH = ssh.SSHClient{
		Hostname:     "10.0.0.4",
		Port:         22222,
		Username:     "root",
		IdentityFile: "/home/user/.ssh/regular_key",
	}
	// Bootstrap SSH hostname is empty → IsInitialized() returns false
	mach.Bootstrap.Kexec.SSHPort = 0
	mach.State.Store(&State{ActiveSSH: SSHTypeKexec})

	result := mach.GetActiveSSH()

	assertion := assert.New(t)
	assertion.Equal("10.0.0.4", result.Hostname, "hostname should come from regular SSH when bootstrap SSH is not initialized")
	assertion.Equal(uint16(22), result.Port, "port should default to 22 when SSHPort is zero")
	assertion.Equal("root", result.Username)
	assertion.Equal("/home/user/.ssh/regular_key", result.IdentityFile)
}

func TestGetActiveSSH_Kexec_BootstrapSSHNotInitialized_CustomPort(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.SSH = ssh.SSHClient{
		Hostname:     "10.0.0.4",
		Port:         22222,
		Username:     "root",
		IdentityFile: "/home/user/.ssh/regular_key",
	}
	mach.Bootstrap.Kexec.SSHPort = 2222
	mach.State.Store(&State{ActiveSSH: SSHTypeKexec})

	result := mach.GetActiveSSH()

	assertion := assert.New(t)
	assertion.Equal("10.0.0.4", result.Hostname)
	assertion.Equal(uint16(2222), result.Port)
	assertion.Equal("/home/user/.ssh/regular_key", result.IdentityFile)
}

func TestGetActiveSSH_Kexec_BootstrapSSHTakesPrecedenceOverRegular(t *testing.T) {
	t.Parallel()

	// Both SSH and Bootstrap.SSH are initialized — Bootstrap.SSH should win.
	mach := newTestMachine()
	mach.SSH = ssh.SSHClient{
		Hostname: "regular-host",
		Port:     22222,
		Username: "regular-user",
	}
	mach.Bootstrap.SSH = ssh.SSHClient{
		Hostname:     "bootstrap-host",
		Port:         22,
		Username:     "bootstrap-user",
		IdentityFile: "/bootstrap_key",
	}
	mach.Bootstrap.Kexec.SSHPort = 22
	mach.State.Store(&State{ActiveSSH: SSHTypeKexec})

	result := mach.GetActiveSSH()

	assertion := assert.New(t)
	assertion.Equal("bootstrap-host", result.Hostname, "bootstrap SSH should take precedence")
	assertion.Equal(uint16(22), result.Port, "port should be overridden by SSHPort")
	assertion.Equal("bootstrap-user", result.Username)
	assertion.Equal("/bootstrap_key", result.IdentityFile)
}

func TestGetActiveSSH_Kexec_SSHPortScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		sshPort      attributes.KexecSSHPort
		expectedPort uint16
	}{
		{"zero value defaults to 22", 0, 22},
		{"explicit 22", 22, 22},
		{"custom 22222", 22222, 22222},
		{"custom 2222", 2222, 2222},
		{"custom 1", 1, 1},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mach := newTestMachine()
			mach.SSH = ssh.SSHClient{Hostname: "10.0.0.5", Port: 22222, Username: "root"}
			mach.Bootstrap.SSH = ssh.SSHClient{
				Hostname:     "10.0.0.5",
				Port:         22,
				Username:     "root",
				IdentityFile: "/key",
			}
			mach.Bootstrap.Kexec.SSHPort = testCase.sshPort
			mach.State.Store(&State{ActiveSSH: SSHTypeKexec})

			result := mach.GetActiveSSH()

			assertion := assert.New(t)
			assertion.Equal(testCase.expectedPort, result.Port)
			assertion.Equal("10.0.0.5", result.Hostname)
		})
	}
}

func TestGetActiveSSH_Kexec_DoesNotMutateOriginalSSHClient(t *testing.T) {
	t.Parallel()

	// GetActiveSSH must return a copy — modifying the returned Port must not
	// affect the original mach.Bootstrap.SSH or mach.SSH.
	mach := newTestMachine()
	mach.Bootstrap.SSH = ssh.SSHClient{
		Hostname:     "10.0.0.6",
		Port:         22,
		Username:     "root",
		IdentityFile: "/key",
	}
	mach.Bootstrap.Kexec.SSHPort = 22222
	mach.State.Store(&State{ActiveSSH: SSHTypeKexec})

	result := mach.GetActiveSSH()
	result.Port = 9999 // mutate the returned copy

	assert.Equal(t, uint16(22), mach.Bootstrap.SSH.Port, "original Bootstrap.SSH.Port must not be mutated")
}

// --- GetActiveSSH: Panic cases ---

func TestGetActiveSSH_PanicsWhenRegularSSHNotInitialized(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	// SSH hostname is empty → IsInitialized() returns false
	mach.State.Store(&State{ActiveSSH: SSHTypeRegular})

	assert.Panics(t, func() {
		mach.GetActiveSSH()
	})
}

func TestGetActiveSSH_PanicsWhenBootstrapSSHNotInitialized(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	// Bootstrap SSH hostname is empty → IsInitialized() returns false
	mach.State.Store(&State{ActiveSSH: SSHTypeBootstrap})

	assert.Panics(t, func() {
		mach.GetActiveSSH()
	})
}

func TestGetActiveSSH_PanicsWhenKexecAndNoSSHInitialized(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	// Neither SSH nor Bootstrap.SSH has hostname set
	mach.Bootstrap.Kexec.SSHPort = 22
	mach.State.Store(&State{ActiveSSH: SSHTypeKexec})

	assert.Panics(t, func() {
		mach.GetActiveSSH()
	})
}

// --- MaybeSudo ---

func TestMaybeSudo_IsRoot(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.MetaInspect.Store(&MetaInspect{IsRoot: true})

	result := mach.MaybeSudo()

	assert.Empty(t, result)
}

func TestMaybeSudo_NotRoot_DefaultSudo(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.MetaInspect.Store(&MetaInspect{IsRoot: false})

	result := mach.MaybeSudo()

	assert.Equal(t, []string{"sudo"}, result)
}

func TestMaybeSudo_CustomSudoProgram(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.SudoProgram = attributes.SudoProgram("doas")
	mach.MetaInspect.Store(&MetaInspect{IsRoot: false})

	result := mach.MaybeSudo()

	assert.Equal(t, []string{"doas"}, result)
}

func TestMaybeSudo_IsRootIgnoresSudoProgram(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.SudoProgram = attributes.SudoProgram("doas")
	mach.MetaInspect.Store(&MetaInspect{IsRoot: true})

	result := mach.MaybeSudo()

	assert.Empty(t, result, "when IsRoot, sudo program is not needed")
}

// --- MaybeBootstrappingPath ---

func TestMaybeBootstrappingPath_Bootstrapped(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.MetaInspect.Store(&MetaInspect{Bootstrapped: true})

	result := mach.MaybeBootstrappingPath("/etc/nixos")

	assert.Equal(t, "/etc/nixos", result)
}

func TestMaybeBootstrappingPath_NotBootstrapped(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.MetaInspect.Store(&MetaInspect{Bootstrapped: false})

	result := mach.MaybeBootstrappingPath("/etc/nixos")

	assert.Equal(t, "/mnt/etc/nixos", result)
}

func TestMaybeBootstrappingPath_EmptyPath(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.MetaInspect.Store(&MetaInspect{Bootstrapped: false})

	result := mach.MaybeBootstrappingPath("")

	assert.Equal(t, "/mnt", result)
}

// --- ValidateSecretsPaths ---

func TestValidateSecretsPaths_NoSecrets(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.Xpath = xpath.New("fleet/flake/cfg/machine")

	err := mach.ValidateSecretsPaths()

	assert.NoError(t, err)
}

func TestValidateSecretsPaths_ExistingFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.key")
	require.NoError(t, os.WriteFile(secretFile, []byte("secret"), 0600))

	mach := newTestMachine()
	mach.Xpath = xpath.New("fleet/flake/cfg/machine")
	mach.Secrets = []attributes.PlainFileOrDirToTransfer{
		{LocalPath: secretFile, RemotePath: "/etc/secret.key"},
	}

	err := mach.ValidateSecretsPaths()

	assert.NoError(t, err)
}

func TestValidateSecretsPaths_ExistingDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	secretDir := filepath.Join(tmpDir, "secrets")
	require.NoError(t, os.Mkdir(secretDir, 0700))

	mach := newTestMachine()
	mach.Xpath = xpath.New("fleet/flake/cfg/machine")
	mach.Secrets = []attributes.PlainFileOrDirToTransfer{
		{LocalPath: secretDir, RemotePath: "/etc/secrets"},
	}

	err := mach.ValidateSecretsPaths()

	assert.NoError(t, err)
}

func TestValidateSecretsPaths_NonExistentFile(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.Xpath = xpath.New("fleet/flake/cfg/machine")
	mach.Secrets = []attributes.PlainFileOrDirToTransfer{
		{LocalPath: "/nonexistent/path/secret.key", RemotePath: "/etc/secret.key"},
	}

	err := mach.ValidateSecretsPaths()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "secrets local path does not exist")
	assert.Contains(t, err.Error(), "/nonexistent/path/secret.key")
}

func TestValidateSecretsPaths_MultipleSecrets_OneMissing(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "existing.key")
	require.NoError(t, os.WriteFile(existingFile, []byte("key"), 0600))

	mach := newTestMachine()
	mach.Xpath = xpath.New("fleet/flake/cfg/machine")
	mach.Secrets = []attributes.PlainFileOrDirToTransfer{
		{LocalPath: existingFile, RemotePath: "/etc/existing.key"},
		{LocalPath: "/nonexistent/missing.key", RemotePath: "/etc/missing.key"},
	}

	err := mach.ValidateSecretsPaths()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing.key")
	assert.NotContains(t, err.Error(), "existing.key")
}

func TestValidateSecretsPaths_MultipleSecrets_AllExist(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "key1")
	file2 := filepath.Join(tmpDir, "key2")

	require.NoError(t, os.WriteFile(file1, []byte("1"), 0600))
	require.NoError(t, os.WriteFile(file2, []byte("2"), 0600))

	mach := newTestMachine()
	mach.Xpath = xpath.New("fleet/flake/cfg/machine")
	mach.Secrets = []attributes.PlainFileOrDirToTransfer{
		{LocalPath: file1, RemotePath: "/etc/key1"},
		{LocalPath: file2, RemotePath: "/etc/key2"},
	}

	err := mach.ValidateSecretsPaths()

	assert.NoError(t, err)
}

// --- ValidateBootstrapSecretsPaths ---

func TestValidateBootstrapSecretsPaths_NoKeys(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.Xpath = xpath.New("fleet/flake/cfg/machine")

	err := mach.ValidateBootstrapSecretsPaths()

	assert.NoError(t, err)
}

func TestValidateBootstrapSecretsPaths_ExistingFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "disk-encryption.key")
	require.NoError(t, os.WriteFile(keyFile, []byte("key"), 0600))

	mach := newTestMachine()
	mach.Xpath = xpath.New("fleet/flake/cfg/machine")
	mach.Bootstrap.DiskEncryptionKeys = []attributes.PlainFileOrDirToTransfer{
		{LocalPath: keyFile, RemotePath: "/tmp/disk-encryption.key"},
	}

	err := mach.ValidateBootstrapSecretsPaths()

	assert.NoError(t, err)
}

func TestValidateBootstrapSecretsPaths_NonExistentFile(t *testing.T) {
	t.Parallel()

	mach := newTestMachine()
	mach.Xpath = xpath.New("fleet/flake/cfg/machine")
	mach.Bootstrap.DiskEncryptionKeys = []attributes.PlainFileOrDirToTransfer{
		{LocalPath: "/nonexistent/disk-encryption.key", RemotePath: "/tmp/disk-encryption.key"},
	}

	err := mach.ValidateBootstrapSecretsPaths()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bootstrap disk encryption key local path does not exist")
	assert.Contains(t, err.Error(), "/nonexistent/disk-encryption.key")
}

func TestValidateBootstrapSecretsPaths_MultipleKeys_AllExist(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	key1 := filepath.Join(tmpDir, "key1")
	key2 := filepath.Join(tmpDir, "key2")

	require.NoError(t, os.WriteFile(key1, []byte("1"), 0600))
	require.NoError(t, os.WriteFile(key2, []byte("2"), 0600))

	mach := newTestMachine()
	mach.Xpath = xpath.New("fleet/flake/cfg/machine")
	mach.Bootstrap.DiskEncryptionKeys = []attributes.PlainFileOrDirToTransfer{
		{LocalPath: key1, RemotePath: "/tmp/key1"},
		{LocalPath: key2, RemotePath: "/tmp/key2"},
	}

	err := mach.ValidateBootstrapSecretsPaths()

	assert.NoError(t, err)
}

func TestValidateBootstrapSecretsPaths_MultipleKeys_OneMissing(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	existingKey := filepath.Join(tmpDir, "existing.key")
	require.NoError(t, os.WriteFile(existingKey, []byte("key"), 0600))

	mach := newTestMachine()
	mach.Xpath = xpath.New("fleet/flake/cfg/machine")
	mach.Bootstrap.DiskEncryptionKeys = []attributes.PlainFileOrDirToTransfer{
		{LocalPath: existingKey, RemotePath: "/tmp/existing.key"},
		{LocalPath: "/nonexistent/missing.key", RemotePath: "/tmp/missing.key"},
	}

	err := mach.ValidateBootstrapSecretsPaths()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing.key")
	assert.NotContains(t, err.Error(), "existing.key")
}

// --- PostUnmarshalInit ---

func TestPostUnmarshalInit_NilLogsAndState(t *testing.T) {
	t.Parallel()

	mach := &Machine{} // Logs and State are nil

	mach.PostUnmarshalInit("", nil)

	assertion := assert.New(t)
	assertion.NotNil(mach.Logs, "Logs should be created")
	assertion.NotNil(mach.State, "State should be created")
}

func TestPostUnmarshalInit_ExistingLogs_GetsInitialized(t *testing.T) {
	t.Parallel()

	// Logs exists but has nil internal fields (e.g. after YAML unmarshal)
	existingLogs := &logs.Logs{}
	mach := &Machine{
		Logs: existingLogs,
	}

	mach.PostUnmarshalInit("", nil)

	assertion := assert.New(t)
	assertion.Same(existingLogs, mach.Logs, "existing Logs pointer should be preserved")
	assertion.NotNil(mach.Logs.PhaseLogs, "PhaseLogs should be initialized")
	assertion.NotNil(mach.Logs.TAS, "TAS should be initialized")
}

func TestPostUnmarshalInit_ExistingLogsAndState_Preserved(t *testing.T) {
	t.Parallel()

	existingLogs := logs.New()
	existingState := atomicpointer.New[State]()
	mach := &Machine{
		Logs:  existingLogs,
		State: existingState,
	}

	mach.PostUnmarshalInit("", nil)

	assertion := assert.New(t)
	assertion.Same(existingLogs, mach.Logs, "existing Logs should not be replaced")
	assertion.Same(existingState, mach.State, "existing State should not be replaced")
}

func TestPostUnmarshalInit_NilState_Only(t *testing.T) {
	t.Parallel()

	mach := &Machine{
		Logs: logs.New(),
	}

	mach.PostUnmarshalInit("", nil)

	assertion := assert.New(t)
	assertion.NotNil(mach.State, "State should be created")
	assertion.NotNil(mach.Logs, "Logs should be preserved")
}

func TestPostUnmarshalInit_NilLogs_ExistingState(t *testing.T) {
	t.Parallel()

	existingState := atomicpointer.New[State]()
	mach := &Machine{
		State: existingState,
	}

	mach.PostUnmarshalInit("", nil)

	assertion := assert.New(t)
	assertion.NotNil(mach.Logs, "Logs should be created when nil")
	assertion.Same(existingState, mach.State, "existing State should be preserved")
}

// --- Init ---

func TestInit_CreatesInternalState(t *testing.T) {
	t.Parallel()

	parent := &attributes.Attributes{
		SudoProgram: attributes.SudoProgram("doas"),
		Tags:        []string{"infrastructure"},
	}

	mach := &Machine{}
	err := mach.Init("test-machine", parent)

	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.NotNil(mach.MetaInspect, "MetaInspect should be created")
	assertion.NotNil(mach.State, "State should be created")
	assertion.NotNil(mach.Logs, "Logs should be created")
	assertion.Equal("test-machine", mach.Name.String())
	assertion.Contains(mach.Tags, "test-machine", "machine name should be added as tag")
	assertion.Contains(mach.Tags, "infrastructure", "parent tags should be inherited")
}

func TestInit_InheritsParentSudoProgram(t *testing.T) {
	t.Parallel()

	parent := &attributes.Attributes{
		SudoProgram: attributes.SudoProgram("doas"),
	}

	mach := &Machine{}
	err := mach.Init("m1", parent)

	require.NoError(t, err)
	assert.Equal(t, "doas", mach.SudoProgram.String())
}

func TestInit_SetsXpath(t *testing.T) {
	t.Parallel()

	parent := &attributes.Attributes{
		Xpath: xpath.New("fleet/my-flake/my-config"),
	}

	mach := &Machine{}
	err := mach.Init("my-machine", parent)

	require.NoError(t, err)
	assert.Equal(t, "fleet/my-flake/my-config/my-machine", mach.Xpath.String())
}

func TestInit_OverwritesExistingInternalState(t *testing.T) {
	t.Parallel()

	// Init should always create fresh internal state, even if some fields
	// were already set (e.g. from a previous partial init or YAML unmarshal).
	oldState := atomicpointer.New[State]()
	oldState.Store(&State{ActiveSSH: SSHTypeBootstrap})

	mach := &Machine{
		State: oldState,
	}

	err := mach.Init("m1", &attributes.Attributes{})

	require.NoError(t, err)
	assert.NotSame(t, oldState, mach.State, "State should be replaced with fresh instance")
}

func TestInit_EmptyName(t *testing.T) {
	t.Parallel()

	parent := &attributes.Attributes{
		Tags:  []string{"parent-tag"},
		Xpath: xpath.New("fleet"),
	}

	mach := &Machine{}
	err := mach.Init("", parent)

	require.NoError(t, err)

	assertion := assert.New(t)
	assertion.Empty(mach.Name.String(), "name should be empty when not provided")
	assertion.NotContains(mach.Tags, "", "empty string should not be added as tag")
	assertion.Equal("fleet", mach.Xpath.String(), "xpath should inherit from parent when name is empty")
}
